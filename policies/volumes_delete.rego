package karavi.volumes.delete

import data.karavi.roles

mydata = output {
  output := roles["roles.json"]
}

myinput = output {
	output := input
}

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
	token == {}
	msg := sprintf("token was invalid", [])
}

default token = {}
token = payload {
	[valid, _, payload] := io.jwt.decode_verify(input.token, {"secret": mydata.secret, "aud": "karavi"})
	valid == true
}