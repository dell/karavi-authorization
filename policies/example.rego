package example

greeting = msg {
  info := opa.runtime()
  hostname := info.env["HOSTNAME"] # Docker sets the HOSTNAME environment variable.
  msg := sprintf("hello from container %q!", [hostname])
}
