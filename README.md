# Mastodon comment server

This implements a simple server that finds the first toot mentioning
a query and all replies to that toot and returns it as json.

## Configuration

The comment server can only be started using systemd socket activation.
The provided systemd units make it listen to the unix domain socket
`/run/mastodon-comments.sock`, on which it will provide fastcgi.


It needs a configuration file as an argument. The configuration file
looks like this:
```json
{
        "url": "https://mastodon.social",
        "token": "<your api token here>"
}
```

## Caching
The server maintains an in-process cache of the root toots. It is
advised that you configure your frontend server to cache the results,
for example, in nginx:

```
location ~ ^/2[0-9][0-9][0-9]/[0-9][0-9]/[0-9][0-9]/[^/]+/comments.json$ {
  fastcgi_pass    unix:/run/mastodon-comments.sock;
  include         fastcgi_params;
  fastcgi_cache GO;
  fastcgi_cache_valid 200 10m;
}
```

The comment server sets a `cache-control` header to cache found comments
for 10 minutes, and no comments for 1 minute, and nginx, if properly
configured will respect that.

## Integration
The server accepts requests in two formats:

1. The query is passed in a "search" query, for example:

   ```<url>?search=/path/to/my/blog/post```

2. If a `search` parameter is not provided, lookup is done based on the path, with the last path element being `comments.json`, for example:

   ```/path/to/my/blog/post/comments.json```

## Status codes

- 500 is returned if any error occured
- 200 is returned otherwise, regardless of whether a toot exists or not

## To do

- Keep an on-disk cache of the list of root toots
- Filter out queries for non-existing posts
- AppArmor profile
