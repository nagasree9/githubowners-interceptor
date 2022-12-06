curl -v \
-H 'X-GitHub-Event: issue_comment' \
-H 'Content-Type: application/json' \
-d '{"action": "created", "issue":{"number": 1}, "repository": {"full_name": "nagasree9/test-owners-interceptor"}, "sender":{"login": "nagasree9"}}' \
http://localhost:8080