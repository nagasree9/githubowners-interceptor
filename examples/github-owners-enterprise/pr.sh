curl -v \
-H 'X-GitHub-Event: pull_request' \
-H 'X-GitHub-Enterprise-Host: github.ford.com' \
-H 'Content-Type: application/json' \
-d '{"action": "opened","number": 7,"repository":{"full_name": "NPATIBAN/test-owners"}, "sender":{"login": "NPATIBAN"}}' \
http://localhost:8080
