curl -v \
-H 'X-GitHub-Event: pull_request' \
-H 'X-GitHub-Enterprise-Host: enterprise-host' \
-H 'Content-Type: application/json' \
-d '{"action": "opened","number": 7,"repository":{"full_name": "org/repo", "clone_url": "https://enterprise-host/org/repo.git"}, "sender":{"login": "nagasree9"}}' \
http://localhost:8080
