curl -v \
-H 'X-GitHub-Event: issue_comment' \
-H 'X-GitHub-Enterprise-Host: github.ford.com' \
-H 'Content-Type: application/json' \
-d '{"action": "created", "issue":{"number": 6}, "repository": {"full_name": "NPATIBAN/test-owners", "clone_url": "https://github.ford.com/NPATIBAN/test-owners.git"}, "sender":{"login": "NPATIBAN"}}' \
http://localhost:8080