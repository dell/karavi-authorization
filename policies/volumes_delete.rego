package karavi.volumes.delete

import data.karavi.common

default response = {
  "allowed": true
}
response = {
  "allowed": false,
  "status": {
  "reason": reason,
  },
} {
  reason = concat(", ", deny)
  reason != ""
}

#
# Ensure there are roles configured.
#
deny[msg] {
  common.roles == {}
  msg := sprintf("no role data found", [])
}

#
# Validate input: token.
#
default token = {}
token = payload {
  [valid, _, payload] := io.jwt.decode_verify(input.token, {"secret": common.secret, "aud": "karavi"})
  valid == true
}
deny[msg] {                                                                                       
  token == {}                                                                                       
  msg := sprintf("token was invalid", [])                                                          
}
