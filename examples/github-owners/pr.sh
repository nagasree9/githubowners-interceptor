curl -v \
-H 'X-GitHub-Event: pull_request' \
-H 'Content-Type: application/json' \
-d '{"action": "opened","number": 2,"repository":{"full_name": "nagasree9/test-owners-interceptor"}, "sender":{"login": "nagasree9"}}' \
http://localhost:8080