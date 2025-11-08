## BudaBit CORS proxy
### A very simplistic CORS proxy to forward git requests from [BudaBit](https://budabit.club) to any git hosting provider
BudaBit is a nostr git client app serving as a nostr community as well as a code collaboration
tool. Thanks to nostr and NIP34, it is able to handle repos and the PR process without GitHub, Gitlab,
or any platforms but is able use them as mere redundant Git servers.

In simple terms: Collaborate with ease, without vendor lock-in.

For this to work, some operations need the right access control permissions and a CORS proxy.
BudaBit is able to handle access-tokens acquired from hosting providers.
Since the tokens are travelling through the CORS proxy along with requests, this is a trusted relationship
between the proxy and the user.

Therefore BudaBit runs this proxy server while allowing users to set their own too.

### Architecture
- This project builds two Docker containers: nginx and the CORS proxy server
- Nginx terminates TLS connections and forwards requests to CORS proxy through http
- CORS proxy makes the request, adds CORS headers and passes it back to Nginx which responds to the browser (budabit)
- Any Access tokens now travel via this proxy
- Nginx handles certificates and possibly rate limiting and such.

### How to use it
1. In docker-compose.yaml set the ALLOWED_ORIGINS env variable for your sites. e.g.:
   - environment:
      - ALLOWED_ORIGINS=https://budabit.club,https://test.budabit.club
2. Test if the proxy is working locally:
    - ``bash 
    export ALLOWED_ORIGINS="localhost"
    go run main.go
    curl "http://localhost:8080/?url=https://api.github.com/users/Pleb5"
    ``
2. Init go module:
    - go mod init example.com/budabit-cors-proxy
    - go mod tidy
3. Upload project to a cheap VPS and install docker and docker-compose on it
4. Point a domain you own to this using a DNS A-record (e.g. corsproxy.budabit.club)
5. You need a cert for this domain of course (letsencrypt)
6. On the VPS cd into project dir and spin up the containers: ``bash docker compose up -d nginx corsproxy``
7. Test the proxy in the browser
    - https://corsproxy.budabit.club/?url=https://api.github.com/users/Pleb5
