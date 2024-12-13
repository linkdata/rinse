# JSON Web Tokens etc

|Glossary||
| -- | -- |
| JWT | JSON Web Token | 
| JWK | JSON Web Key |
 
### JWT
A JSON Web Token (JWT) is a compact URL-safe means of representing claims to be transferred between two parties. The 'claims' are represented in JSON format, and base64url encoded. JWTs are typically used for authorization or information exchange, we use them for the former. JWTs can also be signed, using either a secret (HMAC) or a public/private key pair (RSA or EDSCA). This signature can then be used to verify that the token information itself is unchanged, and also (in the case of public/private keys) that an approved party signed the token.

A JWT basically consists of three parts separated by a period (`.`): the header, payload, and signature. The header contains information like what signing algorithm was used and the payload things like who issued the JWT, its expiration time, etc.

More information about JWTs can be found here: https://jwt.io/introduction.

[!] Note that all information in the header and payload is readable by anyone. A signed JWT is protected against tampering only. 

### JWK 
The JSON Web Key Set (JWKS) is a set of keys which contains the public keys used to verify any JSON Web Token (JWT) issued by the authorization server and signed using the RS256 signing algorithm.


### Sources
- https://jwt.io/
- JWT, JWE, JKW explained https://medium.com/@goynikhil/what-is-jwt-jws-jwe-and-jwk-when-we-should-use-which-token-in-our-business-applications-74ae91f7c96b
- JWK properties https://www.keycloak.org/docs-api/21.1.2/javadocs/constant-values.html
