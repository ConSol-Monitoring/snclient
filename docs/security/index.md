---
linkTitle: Security
---

# Security

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
with the [snclient.pem](snclient.pem).



## Recommendations


### Update Regularly

Always keep SNClient on the latest release version to benefit from security


### Use SSL

Use ssl/tls whenever possible.

    [/settings/default]
    use ssl = true

### TLS 1.3

If possible, set minimum required TLS version to 1.3. Since the number of
clients is limited there is no need to support old browsers.

    [/settings/default]
    tls min version = "tls1.3"


### Allowed Hosts

Using the `allowed hosts` option is a great way to simply block all clients except
your monitoring hosts. This greatly lowers the number of possible attacks.

As a tradeoff between security and maintainability you could add the admin net
here instead of single IPs.

    [/settings/default]
    allowed hosts = 127.0.0.1, ::1, 192.168.56.0/24


### Hashed Password

SNClient supports using hashed passwords so you do not have clear text passwords
in the ini files. Use `snclient hash` to generate a new password hash.

    [/settings/default]
    password = SHA256:9f86d081...


### Allow Nasty Characters

It is recommended to **not** enable `allow nasty characters` as this allows
to exploit existing commands.

    [/settings/default]
    allow nasty characters = false

