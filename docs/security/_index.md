---
title: Security
linkTitle: Security
weight: 600
---

## General

SNClient is written in golang which comes which some benefits regarding security.

- Native TLS/SSL Support

  Since there are no external ssl/tls libraries, SNClient always comes with
  latest TLS support even on legacy systems (if you regularly update SNClient).

- Strong Encryption and Security Standards

  Go's TLS/SSL implementation supports modern encryption and security
  standards, ensuring the confidentiality, integrity, and authenticity of
  data transmitted over the network.

- Secure Ciphers by Default

  Starting with TLS 1.3 Go automatically selects secure ciphers. There is no
  need to set them manually.

## Code Signing

The windows builds (both snclient.exe and the .msi installer) and can be verified
with the signtool.exe from the windows developer sdk, ex.:

```powershell
signtool.exe verify /pa snclient.exe
```

## Recommendations

### Update Regularly

Always keep SNClient on the latest release version to benefit from security

See the [updates page](../updates/) for instructions.

### Use SSL

Use ssl/tls whenever possible.

```ini
[/settings/default]
use ssl = true
```

### TLS 1.3

If possible, set minimum required TLS version to 1.3. Since the number of
clients is limited there is no need to support old browsers.

```ini
[/settings/default]
tls min version = "tls1.3"
```

### Client certificate verification

You can enable client certificate verification using `ca` or `client certificates` options
(both options has the same meaning, `ca` option was added for backward compatibility with NSclient).
You can find example how to generate certificates bellow.

- `certificate` specify server certificate
- `certificate key` specify server key
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

### Allow Nasty Characters

It is recommended to **not** enable `allow nasty characters` as this allows
to exploit existing commands.

```ini
[/settings/default]
allow nasty characters = false
```
