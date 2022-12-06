curl -v \
-H 'X-GitHub-Event: issue_comment' \
-H 'Content-Type: application/json' \
-d '{"action": "created", "issue":{"number": 1}, "repository": {"full_name": "nagasree9/test-owners-interceptor", "clone_url": "https://github.com/nagasree9/test-owners-interceptor.git"}, "sender":{"login": "nagasree9"}}' \
http://localhost:8080