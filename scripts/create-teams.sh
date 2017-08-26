#uaac token owner get bosh_cli  (admin/password on opsmanager)

uaac client add concourse \
  --secret secret \
  --authorized_grant_types "refresh_token password client_credentials" \
  --authorities "bosh.teams.concourse.admin"
