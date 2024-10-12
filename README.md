# Gubber

Gubber is a dockerised tool for backing up github repositories onto a local disk. It automatically keeps a configurable number of backups and deletes old backups as they rotate. Repositories that can no longer be seen on github are kept permanently, and never removed.

Gubber does not keep full backups of repositories for each day, instead generating a bundle and then generating diffs from that "current day" bundle for each day. This is to reduce the amount of data that is stored on the local disk.

Gubber includes a tool for restoring a backup automatically using these diffs, which can be found below.

## Installation

```bash
nano .env #add your gh key, configure variables, see .example.env
docker-compose --env-file .env up -d
```

## Licensing and Contribution

Unless otherwise stated, all contributions will be licensed under the [MIT license](./LICENSE).
