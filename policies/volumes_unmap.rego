package karavi.volumes.unmap

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

deny[msg] {
  common.roles == {}
	msg := sprintf("no role data found", [])
}

deny[msg] {
	token == {}
	msg := sprintf("token was invalid", [])
}

default token = {}
token = payload {
	[valid, _, payload] := io.jwt.decode_verify(input.token, {"secret": common.secret, "aud": "karavi"})
	valid == true
}
