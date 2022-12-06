# githubowners-interceptor

This repository contains an implementation of github-owners cluster interceptor that checks if the `pull_request` sender or the pull_request comment sender with body `/ok-to-test` is a member of organization or repository or the owners file. If either of these returns true the response to trigger is `continue:true`, if neither of these returns true the response to trigger is `continue:false`

## Next Steps

Enhance by variabilizing the comment message (/ok-to-test), so a user can set it at the CRD