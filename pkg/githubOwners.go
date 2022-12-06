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
	"golang.org/x/oauth2"
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
var httpClient *http.Client

var acceptedEventTypes = []string{"pull_request", "issue_comment"}

func (w Interceptor) Process(ctx context.Context, r *triggersv1.InterceptorRequest) *triggersv1.InterceptorResponse {
	headers := interceptors.Canonical(r.Header)
	if v := headers.Get("Content-Type"); v == "application/x-www-form-urlencoded" {
		return interceptors.Fail(codes.InvalidArgument, ErrInvalidContentType.Error())
	}

	p := GitHubOwnersInterceptor{}

	if err := interceptors.UnmarshalParams(r.InterceptorParams, &p); err != nil {
		return interceptors.Failf(codes.InvalidArgument, "failed to parse interceptor params: %v", err)
	}

	actualEvent := headers.Get("X-GitHub-Event")
	enterpriseHost := headers.Get("X-Github-Enterprise-Host")
	// Check if the event type is in the allow-list
	isAllowed := false
	for _, allowedEvent := range acceptedEventTypes {
		if actualEvent == allowedEvent {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return interceptors.Failf(codes.FailedPrecondition, "event type %s is not allowed", actualEvent)
	}
	secretToken, err := w.getSecret(ctx, r, p)
	if err != nil {
		return interceptors.Failf(codes.FailedPrecondition, "error getting the secret: %v", err)
	}

	if secretToken != "" {
		tokenSource := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: secretToken},
		)
		httpClient = oauth2.NewClient(ctx, tokenSource)
	} else {
		httpClient = nil
	}

	if enterpriseHost != "" {
		enterpriseBaseURL := fmt.Sprintf("https://%s", enterpriseHost)
		client, err = gh.NewEnterpriseClient(enterpriseBaseURL, enterpriseBaseURL, httpClient)
		if err != nil {
			return interceptors.Failf(codes.FailedPrecondition, "error initializing a new enterprise client: %v", err)
		}
	} else {
		client = gh.NewClient(httpClient)
	}

	payload, err := parseBody(r.Body, actualEvent)
	if err != nil {
		return interceptors.Failf(codes.FailedPrecondition, "error parsing body: %v", err)
	}

	allowed, err := checkOwnershipAndMembership(ctx, payload, p, client)
	if err != nil {
		return interceptors.Failf(codes.FailedPrecondition, "error checking owner verification: %v", err)
	}

	if allowed {
		return &triggersv1.InterceptorResponse{
			Continue: true,
		}
	}

	commentAllowed, err := allowedOkToTestFromAnOwner(ctx, payload, p, client)
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

func (w Interceptor) getSecret(ctx context.Context, r *triggersv1.InterceptorRequest, p GitHubOwnersInterceptor) (string, error) {
	if p.SecretRef == nil {
		return "", nil
	}
	// Check the secret to see if it is empty
	if p.SecretRef.SecretKey == "" {
		return "", fmt.Errorf("github interceptor secretRef.secretKey is empty")
	}
	ns, _ := triggersv1.ParseTriggerID(r.Context.TriggerID)
	secretToken, err := w.SecretGetter.Get(ctx, ns, p.SecretRef)
	if err != nil {
		return "", fmt.Errorf("error getting secret: %v", err)
	}
	return string(secretToken), nil
}

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
	if eventType == "pull_request" {
		_, ok := jsonMap["number"]
		if !ok && eventType == "pull_request" {
			return results, fmt.Errorf("pull_request body missing 'number' field")
		} else if eventType == "pull_request" {
			prNum = int(jsonMap["number"].(float64))
		} else {
			prNum = -1
		}
	}

	if eventType == "issue_comment" {
		issueSection, ok := jsonMap["issue"].(map[string]interface{})
		if !ok {
			return results, fmt.Errorf("issue_comment body missing 'issue' section")
		}
		_, ok = issueSection["number"]
		if !ok {
			return results, fmt.Errorf("'number' field missing in the issue section of issue_comment body")
		}
		prNum = int(issueSection["number"].(float64))
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

func allowedOkToTestFromAnOwner(ctx context.Context, payload payloadDetails, p GitHubOwnersInterceptor, client *gh.Client) (bool, error) {

	comments, err := getStringPullRequestComment(ctx, payload, client)
	if err != nil {
		return false, err
	}

	for _, comment := range comments {
		payload.Sender = comment.User.GetLogin()
		allowed, err := checkOwnershipAndMembership(ctx, payload, p, client)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

func checkOwnershipAndMembership(ctx context.Context, payload payloadDetails, p GitHubOwnersInterceptor, client *gh.Client) (bool, error) {
	if p.OrgPublicMemberAllowed {
		isUserMemberRepo, err := checkSenderOrgMembership(ctx, payload, client)
		if err != nil {
			return false, err
		}
		if isUserMemberRepo {
			return true, nil
		}
	}
	if p.RepoMemberAllowed {
		checkSenderRepoMembership, err := checkSenderRepoMembership(ctx, payload, client)
		if err != nil {
			return false, err
		}
		if checkSenderRepoMembership {
			return true, nil
		}
	}

	ownerContent, err := getContentFromOwners(ctx, "OWNERS", payload, client)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			// no owner file, skipping
			return false, nil
		}
		return false, err
	}

	return userInOwnerFile(ownerContent, payload.Sender)
}

func checkSenderOrgMembership(ctx context.Context, payload payloadDetails, client *gh.Client) (bool, error) {
	users, resp, err := client.Organizations.ListMembers(ctx, payload.Owner, &gh.ListMembersOptions{
		PublicOnly: true, //we can't list private member in a org
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

func checkSenderRepoMembership(ctx context.Context, payload payloadDetails, client *gh.Client) (bool, error) {
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

func getContentFromOwners(ctx context.Context, path string, payload payloadDetails, client *gh.Client) (string, error) {

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
		if strings.EqualFold(owner, sender) {
			return true, nil
		}
	}
	return false, nil
}

func getStringPullRequestComment(ctx context.Context, payload payloadDetails, client *gh.Client) ([]*gh.PullRequestComment, error) {
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
