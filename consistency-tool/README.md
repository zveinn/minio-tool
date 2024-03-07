# Building the docker contains
```bash
$ docker build --tag check .

```

# Running the list generator
```bash
docker run -v [VOLUME_NAME]:/data -d check:latest /check list -bucket [BUCKET_NAME] -sqldb /data/[DATABASE_FILE] -concurrency 1000 -key [MINIO_ACCESS_KEY] -secret [MINIO_SECRET_KEY] -minio [MINIO_ENDPOINT] --insecure --ignoreMultipart -logN 50000 -reset
```

# Running the checker
```bash
docker run -v [VOLUME_NAME]:/data -d check:latest /check check -sqldb /data/[DATABASE_FILE] -concurrency 1000 -key [MINIO_ACCESS_KEY] -secret [MINIO_SECRET_KEY] -minio [MINIO_ENDPOINT] --insecure --ignoreMultipart -logN 50000 -reset -shards [NUMBER_OF_OBJECT_SHARDS_BASED_ON_ERAZURE_SETTINGS]
```

# WARNING
this will not work if pools have different erazure coding settings.

# How to find number of shards (TODO)
disksPerSet-parity
```bash
$ mc admin info --json mintest | jq ".info.backend"
```

