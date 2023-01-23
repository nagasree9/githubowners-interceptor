## GitHub OWNERS EventListener

Creates an EventListener that listens for pull_request and issue_comment GitHub webhook events. It will return if the listener should trigger a pipeline run or not 

### Try it out locally:

1. To create the GitHub Owner trigger and all related resources, run:

   ```bash
   kubectl apply -f .
   kubectl apply -f ../rbac.yaml
   ```

1. Port forward:

   ```bash
   kubectl port-forward service/el-github-owner-listener 8080
   ```

1. Test by sending the pull request sample payload.

   ```bash
   curl -v \
   -H 'X-GitHub-Event: pull_request' \
   -H 'Content-Type: application/json' \
   -d '{"action": "opened","number": 2,"repository":{"full_name": "nagasree9/test-owners-interceptor"}, "sender":{"login": "nagasree9"}}' \
   http://localhost:8080
   ```

   The response status code should be `202 Accepted`

1. Test by sending the issue_comment sample payload.

   ```bash
   curl -v \
   -H 'X-GitHub-Event: issue_comment' \
   -H 'Content-Type: application/json' \
   -d '{"action": "created", "issue":{"number": 1}, "repository": {"full_name": "nagasree9/test-owners-interceptor"}, "sender":{"login": "nagasree9"}}' \
   http://localhost:8080
   ```

   The response status code should be `202 Accepted`

   `secretKey` is the *given secretToken ex:* `1234567`.

1. You should see result in the logs of interceptor pod:

   ```bash
   kubectl get pods
   kubectl logs <interceptor-pod-name>
   ```
   
   Sample Result Logs from the Interceptor Pod
   True Scenario
   ```bash
   {"level":"info","ts":1670356906.191933,"logger":"fallback","caller":"server/server.go:152","msg":"Interceptor response is: &{Extensions:map[] Continue:true Status:{Code:OK Message:}}"}
   ```
   False Scenario
   ```bash
   {"level":"info","ts":1670356782.2631023,"logger":"fallback","caller":"server/server.go:152","msg":"Interceptor response is: &{Extensions:map[] Continue:false Status:{Code:OK Message:}}"}
   ```

   Taskrun triggers only with True scenario and you can view the taskrun

   ```bash
   kubectl get taskruns
   ```