# plex-lametric

A simple server that returns a Plex Now Playing display for a LaMetric clock.

## Usage

### Go

`$ go get -u github.com/kylegrantlucas/plex-lametric`

`$ env PLEX_HOST=xxxx PLEX_TOKEN=xxxx plex-lametric`

### Docker

`$ docker run -e PLEX_HOST=xxxx -e PLEX_TOKEN=xxxx kylegrantlucas/plex-lametric`

### Docker Compose

```yaml
plex-lametric:
    container_name: plex-lametric
    image: kylegrantlucas/plex-lametric
    environment:
      - PLEX_HOST=xxxx
      - PLEX_TOKEN=xxxx
    restart: unless-stopped
```