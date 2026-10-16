---
title: Security
linkTitle: Security
weight: 600
---

## General

SNClient is written in Go, which offers several security benefits.

- Native TLS/SSL Support

  Since there are no external TLS/SSL libraries, SNClient always comes with
  the latest TLS support even on legacy systems (if you regularly update SNClient).

- Strong Encryption and Security Standards

  Go's TLS/SSL implementation supports modern encryption and security
  standards, ensuring the confidentiality, integrity, and authenticity of
  data transmitted over the network.

- Secure Ciphers by Default

  Starting with TLS 1.3 Go automatically selects secure ciphers. There is no
  need to set them manually.

## Code Signing

The Windows builds (both snclient.exe and the .msi installer) can be verified
with signtool.exe from the Windows SDK, for example:

```powershell
signtool.exe verify /pa snclient.exe
```

## Recommendations

### Update Regularly

Always keep SNClient on the latest release version to benefit from security fixes.

See the [updates page](../updates/) for instructions.

### Use SSL

Use TLS/SSL whenever possible.

```ini
[/settings/default]
use ssl = true
```

### HSTS Header

SNClient supports sending HTTP Strict Transport Security (HSTS) headers.
This is disabled by default, since this addresses browser and human interaction
issues which usually do not apply to the SNClient scenarios.

The HSTS header tells browsers to automatically use HTTPS for future connections
to the server. SNClient is usually accessed by monitoring tools rather than browsers,
so this header generally has limited relevance.

You can enable it if required by a security audit. Make sure you
use valid certificates before enabling it.

The header is set to:

- `max-age=31536000` (1 year)

To disable HSTS headers in case this creates issues somehow:

```ini
[/settings/WEB/server]
use hsts header = false
```

Or to disable it for all HTTP servers:

```ini
[/settings/default]
use hsts header = false
```

### TLS 1.3

If possible, set the minimum required TLS version to 1.3. Since the number of
clients is limited there is no need to support old browsers.

```ini
[/settings/default]
tls min version = "tls1.3"
```

### Client certificate verification

You can enable client certificate verification using `ca` or `client certificates` options
(both options have the same meaning; the `ca` option was added for backward compatibility with NSClient++).
You can find an example of how to generate certificates below.

- `certificate` specifies the server certificate
- `certificate key` specifies the server key
- `ca` and/or `client certificates` specify the CA certificate for certificate verification and enable verification itself.

```ini
[/settings/default]
certificate = ${certificate-path}/server.crt
certificate key = ${certificate-path}/server.key
client certificates = ${certificate-path}/ca.pem
;ca = ${certificate-path}/ca.pem
```

#### Certificate generation example

1. Generate CA certificate

   ```bash
   # generate CA key
   openssl genrsa -aes256 -out ca/ca.key 4096
   # generate CA certificate
   openssl req -x509 -new -nodes -key ca/ca.key -sha256 -days 7500 -out ca/ca.pem -subj "/C=US/L=New York/O=Company/CN=My CA"
   ```

2. Generate client certificate

   ```bash
   # generate client key
   openssl genrsa -out client1.key 4096
   # generate client certificate request
   openssl req -new -key client1.key -out client1.csr -subj "/C=US/L=New York/O=Company/CN=Client"
   # sign client certificate by CA
   openssl x509 -req -in client1.csr -out client1.pem -CA ca.pem -CAkey ca/ca.key -CAcreateserial -days 7300 -sha256
   ```

3. Generate server certificate

   ```bash
   # generate server key
   openssl genrsa -out server.key 4096
   # generate server certificate request
   openssl req -new -key server.key -out server.csr -subj "/C=US/L=New York/O=Company/CN=Server"
   # sign server certificate by CA
   openssl x509 -req -in server.csr -out server.pem -CA ca.pem -CAkey ca/ca.key -CAcreateserial -days 7300 -sha256
   ```

### Allowed Hosts

Using the `allowed hosts` option is a great way to simply block all clients except
your monitoring hosts. This greatly lowers the number of possible attacks.

As a tradeoff between security and maintainability you could add the admin net
here instead of single IPs.

```ini
[/settings/default]
allowed hosts = 127.0.0.1, ::1, 192.168.56.0/24
```

### Hashed Password

SNClient supports using hashed passwords so you do not have clear text passwords
in the ini files. Use `snclient hash` to generate a new password hash.

```ini
[/settings/default]
password = SHA256:9f86d081...
```

### External Commands

Using external commands is safe under certain conditions.

- [allow arguments](#allow-arguments) is disabled
- [allow nasty characters](#allow-nasty-characters) is unchanged

Changing nasty characters is almost always a bad idea. Allowing arguments can
be necessary in certain scenarios, read more in the [allow arguments](#allow-arguments) section.

### Allow Nasty Characters

It is recommended to **not** enable `allow nasty characters` or `allow control
characters` as this allows exploitation of existing commands.

Removing some of the `nasty characters` is also not a good idea. Each of them
is there for a reason and removing any of them probably allows command injection
vulnerabilities.

```ini
[/settings/default]
allow nasty characters = false
allow control characters = false
```

### Allow Arguments

Normally arguments for external commands are not allowed because they are forwarded
to the shell which makes them easy to exploit.

When enabling arguments, make sure to use quoted arguments like `$ARGS"$`. Using unquoted
arguments leads to command injection vulnerabilities.

A better approach would be to use aliased commands.
