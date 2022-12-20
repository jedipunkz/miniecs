# miniecs

miniecs is a CLI tool that allows you to fuzzy finder incremental search your ecs envs and login to your container. And you can execute any commands on your container.

![Go-CI](https://github.com/jedipunkz/miniecs/workflows/Go-CI/badge.svg)
![CodeQL](https://github.com/jedipunkz/miniecs/workflows/CodeQL/badge.svg)

<img src="https://raw.githubusercontent.com/jedipunkz/miniecs/main/pix/miniecs.gif">

## Requirement

- install go 1.17.x or later
- install [session-manager-plugin](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)

## Installation

### Homebrew

```shell
brew tap jedipunkz/miniecs
brew install jedipunkz/miniecs/miniecs
```

### Go Install

```shell
go install github.com/jedipunkz/miniecs@latest
```

## Usage

### Login Sub-Command

A sub-command to login ecs container with incremental searching.

Run 'login' sub-command to login to container. If you don't specify cluster, miniecs find all of your clusters in region. 'region' option is requred.

```shell
$ miniecs login --region <REGION_NAME>
```

You can also specify a cluster and shell. These options are optional. Default shell is 'sh'.

```shell
$ miniecs login --region <REGION_NAME> --cluster <CLUSTER_NAME> --shell <SHELL>
```

### Execute Sub-Command

A sub-command to execute command in container.

```shell
$ miniecs exec \
    --region    <REGION_NAME> \
    --cluster   <CLUSTER_NAME> \
    --service   <SERVICE_NAME> \
    --container <CONTAINER_NAME> \
    --command   <SHELL_COMMAND>
```

### List Sub-Command

A sub-command to get table information of ecs cluster(s), service(s) and container(s).

```shell
$ miniecs list --region <REGION_NAME>
```

## License

[Apache License 2.0](https://github.com/jedipunkz/awscreds/blob/main/LICENSE)

## Author

[jedipunkz](https://twitter.com/jedipunkz)
