$Headers = @{
  # "X-GitHub-Event" = "pull_request"
  "X-GitHub-Event" = "issue_comment"
  "X-Hub-Signature" = "sha1=33035a3a8b7b395139881c2654b59cd1e50ab770"
  ""
}
# $Body = '{"action": "opened","number": 4,"repository":{"full_name": "IaC/test-owners"}, "sender":{"login": "NPATIBAN"}}'
$Body = '{"action": "created", "issue":{"number": 5}, "repository": {"full_name": "IaC/test-owners"}, "sender":{"login": "NPATIBA"}}'

Invoke-RestMethod -Uri "http://localhost:8080" -Headers $Headers -Body $Body -ContentType "application/json"