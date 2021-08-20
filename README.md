# Raw Data

A simple sqlite store for raw data. 

Its provide a thin rest api for differents namespaces. Each namespace belongs to a sqlite's db. 

This project pretends to be simple. Having a sqlite store allows move files directly when needed.

A fileserver is embebed for that purpose, and the option to take a snapshot for each namespace.

Future work could include a sharding strategy to split load, and a index for text data. 

:sparkles: **New** If `-stream` option is selected, it will stream each new entry by namespace in a Redis Instance. 


## Use cases

For small data ~1mb. 

My use case is to store crawled data (~700kb), up to 500k objects per namespace.

Bigger files are discourage. Each file is loaded in memory for each request. SQLite doesn't provide a way to stream data directly. 

## Defaults to be considered

1. A `default` namespace is created when started. 
2. No auth, [reverse proxy auth](https://docs.nginx.com/nginx/admin-guide/security-controls/configuring-subrequest-authentication/) is easy to be included using nginx. In the future could be included as a auth endpoint in the app.
3. Every object is compressed and decompressed using zlib.

Also check the default config values:

```
var (
	rateLimit    = Env("RD_RATE_LIMIT", "1000")
	listenAddr   = Env("RD_LISTEN_ADDR", ":6667")
	nsDir        = Env("RD_NS_DIR", "data/")
	redisAddr    = Env("RD_REDIS_ADDR", "localhost:6379")
	redisPass    = Env("RD_REDIS_PASS", "")
	redisDB      = Env("RD_REDIS_DB", "0")
	streamNo     = Env("RD_STREAM", "false")
	eStreamLimit = Env("RD_STREAM_LIMIT", "1000")
)
```


## API

- GET /status
  - 200 if everything is ok

- GET /files
  - Fileserver. List all the sqlite files for each namespace
  
- POST /v1/namespace
  - Create a namespace
  { "name" : "my_namespace" }

- GET /v1/namespace
  - List namespaces

- GET /v1/namespace/{namespace}/_backup 
  - Takes a backup, This action is SYNC, so consider the time of the request for big files ( > 6 GB)
  
- GET /v1/data/{namespace} 
  - List files as an API, base64 encoded data.
  
- GET /v1/data/{namespace}/_list 
  - List only IDs and created fields
  
- PUT /{namespace}/{key}
  - 201 if created, anything else = fail
  - If the path already exist, the data will be replaced with the new sent.
  
- POST /{namespace}/{key}
  - 201 if created, anything else = fail

- DELETE /{namespace}/{key}
  - 200 Deleted
  
- GET /{namespace}/ *will be removed in the next release*
  - List files as an API, base64 encoded data.
  - This should be moved to the API endpoints. Filter options will be included
  in future versions.


## Usage

By default `default` namespace is created: 

Create or update a object
```
curl -v -L -X PUT -d bigswag localhost:6667/default/wehave
```

Create a new object
```
curl -v -L -X PUT -d bigswag localhost:6667/default/wehave
```

Get object (uncompressed original format)
```
curl -v -L localhost:6667/default/wehave
```

Delete object
```
curl -v -L -X DELETE localhost:6667/default/wehave
```


## Similar projects and inspiration for this work

1. https://github.com/geohot/minikeyvalue
It use a Go Web server as coordinator that manage keys in a leveldb store and redirect each request to a Nginx server used as volume. 
The problem is the same than before, is not suitable for small data in the long term, but I take the idea to have a easy way to restore the data if something fails. 

2. Some time later, I found that the previous work was based on [this paper](https://www.usenix.org/legacy/event/osdi10/tech/full_papers/Beaver.pdf) 

3. https://github.com/chrislusf/seaweedfs 


4. Google: http://infolab.stanford.edu/~backrub/google.html


## References

1. [Building your data lake](https://cloudblogs.microsoft.com/industry-blog/en-gb/technetuk/2020/04/09/building-your-data-lake-on-azure-data-lake-storage-gen2-part-1/)
2. [Data Lake](https://en.wikipedia.org/wiki/Data_lake) 
3. [Data block in HDFS](https://data-flair.training/blogs/data-block/)
4. [Facebook photo storage](https://www.usenix.org/legacy/event/osdi10/tech/full_papers/Beaver.pdf)
5. [The anatomy of a Large-Scale web search engine](http://infolab.stanford.edu/~backrub/google.html)
8. [When to use sqlite](https://www.sqlite.org/whentouse.html)
9. [35% faster than the filesystem](https://www.sqlite.org/fasterthanfs.html)

*Redis INFO*

- https://redis.io/topics/streams-intro
- https://redis.com/blog/beyond-the-cache-with-python/
- https://redis-py.readthedocs.io/en/stable/
- https://tirkarthi.github.io/programming/2018/08/20/redis-streams-python.html 
