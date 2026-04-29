# Platform Notes

## Source Address and Device

`source_address` and `source_device` are local-stack controls. They apply to local traceroute, MTR, MTU, and speed tools when the OS supports the underlying behavior.

They do not apply to Globalping.

## TOS / Traffic Class

Use `tos` only with local traceroute/MTR tools.

Do not pass `tos` to:

- Globalping tools
- MTU tool

## Packet Size

`packet_size` is local traceroute/MTR input. It means total bytes including IP and active probe protocol headers.

Do not pass `packet_size` to:

- Globalping tools
- MTU tool

## Windows

Windows device selection is source-address-based for many paths. Treat `source_device` as a hint unless the returned source path proves otherwise.

## Privileges

Raw socket operations may require elevated privileges or platform-specific packet capture/runtime support. If MCP returns a permission error, ask the user whether to rerun NextTrace with the required privileges.
