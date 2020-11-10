package karavi.volumes.create

mydata = output {
  output := data["default"]["roles"]["roles.json"]
}

myinput = output {
	output := input
}

default quota = 0
quota = mydata.roles[token.role].quota

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

deny[msg] {
  not mydata.roles[token.role]
	msg := sprintf("unknown role: %q", [token.role])
}

deny[msg] {
	input.storagepool != mydata.roles[token.role].pools[_]
  msg := sprintf("role %q does not permit access to pool %q", [token.role, input.storagepool])
} 

deny[msg] {
	role := token.role
  quota := mydata.roles[role].quota
  cap := to_number(input.request.volumeSizeInKb)
	cap > quota
  msg := sprintf("requested capacity %v exceeds quota %v for role %q", [format_int(cap,10), format_int(quota,10), role])
}

default token = {}
token = payload {
	[valid, _, payload] := io.jwt.decode_verify(input.token, {"secret": mydata.secret, "aud": "karavi"})
	valid == true
}
