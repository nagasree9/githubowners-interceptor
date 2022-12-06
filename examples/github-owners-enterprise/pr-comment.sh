curl -v \
-H 'X-GitHub-Event: issue_comment' \
-H 'X-GitHub-Enterprise-Host: github.ford.com' \
-H 'Content-Type: application/json' \
-d '{"action": "created", "issue":{"number": 6}, "repository": {"full_name": "NPATIBAN/test-owners"}, "sender":{"login": "NPATIBAN"}}' \
http://localhost:8080