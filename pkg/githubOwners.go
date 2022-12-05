package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"sigs.k8s.io/yaml"

	gh "github.com/google/go-github/v31/github"
	triggersv1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
	"github.com/tektoncd/triggers/pkg/interceptors"
	"google.golang.org/grpc/codes"
)

var _ triggersv1.InterceptorInterface = (*Interceptor)(nil)

// ErrInvalidContentType is returned when the content-type is not a JSON body.
var ErrInvalidContentType = errors.New("form parameter encoding not supported, please change the hook to send JSON payloads")

type Interceptor struct {
	SecretGetter interceptors.SecretGetter
}

const OKToTestCommentRegexp = `(^|\n)\/ok-to-test(\r\n|\r|\n|$)`

var client *gh.Client

func (w *Interceptor) Process(ctx context.Context, r *triggersv1.InterceptorRequest) *triggersv1.InterceptorResponse {
	headers := interceptors.Canonical(r.Header)
	if v := headers.Get("Content-Type"); v == "application/x-www-form-urlencoded" {
		return interceptors.Fail(codes.InvalidArgument, ErrInvalidContentType.Error())
	}

	p := triggersv1.GitHubInterceptor{}
	if err := interceptors.UnmarshalParams(r.InterceptorParams, &p); err != nil {
		return interceptors.Failf(codes.InvalidArgument, "failed to parse interceptor params: %v", err)
	}

	actualEvent := headers.Get("X-GitHub-Event")
	// Check if the event type is in the allow-list
	if p.EventTypes != nil {
		isAllowed := false
		for _, allowedEvent := range p.EventTypes {
			if actualEvent == allowedEvent {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return interceptors.Failf(codes.FailedPrecondition, "event type %s is not allowed", actualEvent)
		}
	}

	// header := headers.Get("X-Hub-Signature-256")
	// if header == "" {
	// 	header = headers.Get("X-Hub-Signature")
	// }
	// if header == "" {
	// 	return interceptors.Fail(codes.FailedPrecondition, "Must set X-Hub-Signature-256 or X-Hub-Signature header")
	// }

	// secretToken, err := w.getSecret(ctx, r, p)
	// if err != nil {
	// 	return interceptors.Failf(codes.FailedPrecondition, "error validating the secret: %v", err)
	// }

	// if err := gh.ValidateSignature(header, []byte(r.Body), []byte(secretToken)); err != nil {
	// 	return interceptors.Fail(codes.FailedPrecondition, err.Error())
	// }

	payload, err := parseBody(r.Body, actualEvent)
	if err != nil {
		return interceptors.Failf(codes.FailedPrecondition, "error parsing body: %v", err)
	}

	allowed, err := checkOwnershipAndMembership(ctx, payload)
	if err != nil {
		return interceptors.Failf(codes.FailedPrecondition, "error checking owner verification: %v", err)
	}

	if allowed {
		return &triggersv1.InterceptorResponse{
			Continue: true,
		}
	}

	commentAllowed, err := allowedOkToTestFromAnOwner(ctx, payload)
	if err != nil {
		return interceptors.Failf(codes.FailedPrecondition, "error checking comments for verification: %v", err)
	}
	if commentAllowed {
		return &triggersv1.InterceptorResponse{
			Continue: true,
		}
	}
	return &triggersv1.InterceptorResponse{
		Continue: false,
	}
}

// func (w *Interceptor) getSecret(ctx context.Context, r *triggersv1.InterceptorRequest, p triggersv1.GitHubInterceptor) (string, error) {
// 	if p.SecretRef != nil {
// 		return "", nil
// 	}
// 	// Check the secret to see if it is empty
// 	if p.SecretRef.SecretKey == "" {
// 		return "", fmt.Errorf("github interceptor secretRef.secretKey is empty")
// 	}

// 	ns, _ := triggersv1.ParseTriggerID(r.Context.TriggerID)
// 	secretToken, err := w.SecretGetter.Get(ctx, ns, p.SecretRef)
// 	if err != nil {
// 		return "", fmt.Errorf("error getting secret: %v", err)
// 	}
// 	return string(secretToken), nil
// }

func parseBody(body string, eventType string) (payloadDetails, error) {
	results := payloadDetails{}
	if body == "" {
		return results, fmt.Errorf("body is empty")
	}
	var jsonMap map[string]interface{}
	err := json.Unmarshal([]byte(body), &jsonMap)
	if err != nil {
		return results, err
	}

	var prNum int
	_, ok := jsonMap["number"]
	if !ok && eventType == "pull_request" {
		return results, fmt.Errorf("pull_request body missing 'number' field")
	} else if eventType == "pull_request" {
		prNum = int(jsonMap["number"].(float64))
	} else {
		prNum = -1
	}

	repoSection, ok := jsonMap["repository"].(map[string]interface{})
	if !ok {
		return results, fmt.Errorf("payload body missing 'repository' field")
	}

	fullName, ok := repoSection["full_name"].(string)
	if !ok {
		return results, fmt.Errorf("payload body missing 'repository.full_name' field")
	}

	senderSection, ok := jsonMap["sender"].(map[string]interface{})
	if !ok {
		return results, fmt.Errorf("payload body missing 'sender' field")
	}
	prSender, _ := senderSection["login"].(string)

	results = payloadDetails{
		PrNumber:   prNum,
		Sender:     prSender,
		Owner:      strings.Split(fullName, "/")[0],
		Repository: strings.Split(fullName, "/")[1],
	}

	return results, nil
}

func allowedOkToTestFromAnOwner(ctx context.Context, payload payloadDetails) (bool, error) {

	comments, err := getStringPullRequestComment(ctx, payload)
	if err != nil {
		return false, err
	}

	for _, comment := range comments {
		payload.Sender = comment.User.GetLogin()
		allowed, err := checkOwnershipAndMembership(ctx, payload)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func checkOwnershipAndMembership(ctx context.Context, payload payloadDetails) (bool, error) {
	isUserMemberRepo, err := checkSenderOrgMembership(ctx, payload)
	if err != nil {
		return false, err
	}
	if isUserMemberRepo {
		return true, nil
	}

	checkSenderRepoMembership, err := checkSenderRepoMembership(ctx, payload)
	if err != nil {
		return false, err
	}
	if checkSenderRepoMembership {
		return true, nil
	}

	ownerContent, err := getContentFromOwners(ctx, "OWNERS", payload)
	if err != nil {
		if strings.Contains(err.Error(), "cannot find") {
			// no owner file, skipping
			return false, nil
		}
		return false, err
	}

	return userInOwnerFile(ownerContent, payload.Sender)
}

func checkSenderOrgMembership(ctx context.Context, payload payloadDetails) (bool, error) {
	users, resp, err := client.Organizations.ListMembers(ctx, payload.Owner, &gh.ListMembersOptions{
		PublicOnly: true,
	})
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		return false, nil
	}

	if err != nil {
		return false, err
	}
	for _, user := range users {
		if user.GetLogin() == payload.Sender {
			return true, nil
		}
	}
	return false, nil
}

func checkSenderRepoMembership(ctx context.Context, payload payloadDetails) (bool, error) {
	users, _, err := client.Repositories.ListCollaborators(ctx, payload.Owner, payload.Repository, &gh.ListCollaboratorsOptions{})
	if err != nil {
		return false, err
	}

	for _, user := range users {
		if user.GetLogin() == payload.Sender {
			return true, nil
		}
	}

	return false, nil
}

func getContentFromOwners(ctx context.Context, path string, payload payloadDetails) (string, error) {

	fileContent, directoryContent, _, err := client.Repositories.GetContents(ctx, payload.Owner, payload.Repository, path, &gh.RepositoryContentGetOptions{})

	if err != nil {
		return "", err
	}

	if directoryContent != nil {
		return "", fmt.Errorf("referenced file inside the Github Repository %s is a directory", path)
	}

	fileData, err := fileContent.GetContent()

	if err != nil {
		return "", err
	}

	return fileData, nil
}

func userInOwnerFile(ownerContent, sender string) (bool, error) {
	oc := ownersConfig{}
	err := yaml.Unmarshal([]byte(ownerContent), &oc)
	if err != nil {
		return false, err
	}

	for _, owner := range append(oc.Approvers, oc.Reviewers...) {
		if owner == sender {
			return true, nil
		}
	}
	return false, nil
}

func getStringPullRequestComment(ctx context.Context, payload payloadDetails) ([]*gh.PullRequestComment, error) {
	var ret []*gh.PullRequestComment
	comments, _, err := client.PullRequests.ListComments(ctx, payload.Owner, payload.Repository, payload.PrNumber, &gh.PullRequestListCommentsOptions{})
	if err != nil {
		return ret, err
	}
	for _, comment := range comments {
		if MatchRegexp(OKToTestCommentRegexp, string(*comment.Body)) {
			ret = append(ret, comment)
		}
	}
	return ret, nil
}

func MatchRegexp(reg, comment string) bool {
	re := regexp.MustCompile(reg)
	return string(re.Find([]byte(comment))) != ""
}
