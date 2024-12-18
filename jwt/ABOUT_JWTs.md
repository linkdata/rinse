# Module for handling JSON Web Tokens etc
|Glossary||
| -- | -- |
| JWT | JSON Web Token | 
| JWK | JSON Web Key |
 
### What is JWT
A JSON Web Token (JWT) is a compact URL-safe means of representing claims to be transferred between two parties. The 'claims' are represented in JSON format, and base64url encoded*. JWTs are typically used for authorization or information exchange, we use them for the former. JWTs can also be signed, using either a secret (HMAC) or a public/private key pair (RSA or EDSCA*). This signature can then be used to verify that the token information itself is unchanged, and also (in the case of public/private keys) that an approved party signed the token.

A JWT basically consists of three parts separated by a period (`.`): the header, payload, and signature. The header contains information like what signing algorithm was used and the payload things like who issued the JWT, its expiration time, etc.

This module expects the following claims in header and payload:

| Header |||
|--|--|--|
| *Claim* | *Format* | *Description* |
| kid | String | Key id |
| alg | String | Signing algorithm |


| Payload |||
|--|--|--|
| *Claim* | *Format* | *Description* |
| preferred_username | String | Username |
| exp | Int64 (UNIX timestamp) | Expiration date |


More information about JWTs can be found here: https://jwt.io/introduction.

\* Note that all information in the header and payload is readable by anyone. A signed JWT is protected against tampering only.

\** Only RSA is supported in this module.

### What is JWK 
The JSON Web Key Set (JWKS) is a set of keys which contains the public keys used to verify any JSON Web Token (JWT) issued by the authorization server and signed using the RS256 signing algorithm.

### Authorization servers
The authorization server is the party that issues JWTs, and provides JWKs (if public/private key signing is used). 

Often this server has an endpoint that can be used for fetching its JWKs. This module expects the endpoint to return a JSON that looks something like this: 
```
{
  "keys":[
    {
        "kid":"A1bC2dE3fH4iJ5kL6mN7oP8qR9sT0u",
        "x5c":["MIID..."]
    },
    {
        "kid":"C2dE3fH4iJ5kL6mN7oP8qR9sT0uV1w",
        "x5c":["MIID..."]
    },
    ...
  ]
}
```
The keys may have more properties (i.e. `kty`, `alg`, `use`), but `kid` and `x5c` are the only necessary ones.

Keycloak and Microsoft Entra both follow this formula. 

[!] Keycloak has been tested, while Entra has not. 

### Further reading
- https://jwt.io/
- JWT, JWE, JKW explained https://medium.com/@goynikhil/what-is-jwt-jws-jwe-and-jwk-when-we-should-use-which-token-in-our-business-applications-74ae91f7c96b
- JWK properties https://stytch.com/blog/understanding-jwks/
- RFC7515 (JSON Web Signature standard) https://www.rfc-editor.org/rfc/rfc7515
