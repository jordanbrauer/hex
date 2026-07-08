# hex/disk v1 ships local backend only

`hex/disk` defines a `Disk` interface (Laravel Storage-style: read, write, exists, delete, list, url) and a single concrete backend for the local filesystem. Cloud backends (`hex/disk/s3`, `hex/disk/minio`, `hex/disk/gcs`) come later as opt-in subpackages, following the same pattern as `hex/db/sqlite` / `hex/db/postgres`.

We rejected shipping S3 alongside local in v1 because it adds AWS SDK weight to every consumer immediately, and it locks the interface shape before we've stress-tested it against real local use. Getting the local backend and the interface right first means the S3 (or minio, or GCS) driver only has to satisfy an already-proven contract.

The interface will explicitly support the affordances Laravel's Storage facade provides (streamed reads/writes, temporary URLs, visibility) so the S3 driver, when it lands, does not need interface changes.
