---
title: Security
linkTitle: Security
weight: 600
---

## General

SNClient is written in Go, which provides these security features:

- Native TLS/SSL Support

  SNClient does not use external TLS/SSL libraries. Regular SNClient updates provide the latest TLS support, even on legacy systems.

- Strong Encryption and Security Standards

  Go's TLS/SSL implementation supports modern encryption and security standards.
  These protect the confidentiality, integrity, and authenticity of data sent over the network.

- Secure Ciphers by Default

  Starting with TLS 1.3, Go selects secure ciphers automatically. You do not need to set them manually.

## Code Signing

Use signtool.exe from the Windows SDK to verify Windows builds, both snclient.exe and the .msi installer. For example:

```powershell
signtool.exe verify /pa snclient.exe
```

## Recommendations

### Update Regularly

Keep SNClient at the latest release to receive security fixes.

See the [updates page](../updates/) for instructions.

### Use SSL

Use TLS/SSL whenever possible.

```ini
[/settings/default]
use ssl = true
```

### HSTS Header

SNClient can send HTTP Strict Transport Security (HSTS) headers. HSTS is disabled by default.
The header tells browsers to use HTTPS automatically for future connections to the server.
Monitoring tools usually access SNClient, so this browser feature generally has limited relevance.

If a security audit requires HSTS, you can enable it. Before you enable HSTS, make sure that your certificates are valid.

The header is set to:

- `max-age=31536000` (1 year)

If HSTS causes problems, disable it for the web server:

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

If possible, set the minimum required TLS version to 1.3.
SNClient has a limited set of clients, so it does not need to support old browsers.

```ini
[/settings/default]
tls min version = "tls1.3"
```

### Client certificate verification

Use `ca` or `client certificates` to enable client certificate verification. Both options have the same meaning.
The `ca` option provides backward compatibility with NSClient++.
The following example shows how to generate certificates.

- `certificate` specifies the server certificate
- `certificate key` specifies the server key
- `ca` and/or `client certificates` specify the CA certificate and enable certificate verification.

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

Use `allowed hosts` to block all clients except your monitoring hosts. This reduces the number of possible attacks.

To balance security and maintenance, you can specify the admin network instead of individual IP addresses.

```ini
[/settings/default]
allowed hosts = 127.0.0.1, ::1, 192.168.56.0/24
```

### Hashed Password

SNClient supports hashed passwords to avoid clear text passwords in INI files.
Use `snclient hash` to generate a password hash.

```ini
[/settings/default]
password = SHA256:9f86d081...
```

### External Commands

Using external commands is safe under certain conditions.

- [allow arguments](#allow-arguments) is disabled
- [allow nasty characters](#allow-nasty-characters) is unchanged

Do not change the nasty characters setting. Some scenarios require arguments. See [allow arguments](#allow-arguments) for details.

### Allow Nasty Characters

Do not enable `allow nasty characters` or `allow control
characters`. These settings allow attackers to exploit existing commands.

Do not remove entries from `nasty characters`. Each entry protects against attacks.
Removing any entry probably allows command injection vulnerabilities.

```ini
[/settings/default]
allow nasty characters = false
allow control characters = false
```

### Allow Arguments

By default, SNClient blocks arguments for external commands. It forwards these arguments to the shell, where attackers can easily exploit them.

If you enable arguments, use quoted arguments like `$ARGS"$`.
Unquoted arguments lead to command injection vulnerabilities.

Prefer aliased commands.
