# miniecs

miniecs is a cli tool to login ecs container with fuzzy finder incremental search.

![Go-CI](https://github.com/jedipunkz/miniecs/workflows/Go-CI/badge.svg)
![CodeQL](https://github.com/jedipunkz/miniecs/workflows/CodeQL/badge.svg)

<img src="https://raw.githubusercontent.com/jedipunkz/miniecs/main/pix/miniecs.gif">

## Requirements

- Install Go 1.22.x or later
- Install [session-manager-plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)

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

A sub-command to login to an ECS container with incremental searching.

Run the 'login' sub-command to log in to a container. If you don't specify a cluster, miniecs will find all of your clusters in the region. The 'region' option is required.

```shell
$ miniecs login --region <REGION_NAME>
```

You can also specify a cluster and shell. These options are optional. The default shell is 'sh'.

```shell
$ miniecs login --region <REGION_NAME> --cluster <CLUSTER_NAME> --shell <SHELL>
```

## License

[Apache License 2.0](https://github.com/jedipunkz/awscreds/blob/main/LICENSE)

## Author

[jedipunkz](https://twitter.com/jedipunkz)