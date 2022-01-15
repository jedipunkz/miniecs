# miniecs

miniecs is a CLI tool that allows you to fuzzy finder incremental search your ecs envs and login to your container. And you can execute any commands on your container.

[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/jedipunkz/miniecs/Go-CI?style=flat-square)](https://github.com/jedipunkz/miniecs/actions?query=workflow%3AGo-CI)

<img src="https://raw.githubusercontent.com/jedipunkz/miniecs/main/pix/miniecs.gif">

## Requirement

- install go 1.17.x or later
- install [session-manager-plugin](https://docs.aws.amazon.com/ja_jp/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)

## Installation

```shell
go install github.com/jedipunkz/miniecs@latest
```

## Usage

### Login Sub-Command

A sub-command to login ecs container with incremental searching.
You must to have `~/miniecs.yaml` file included ecs resource(s) infomation.

```yaml
shell: sh # or shell name on container
```

Run 'login' sub-command.

```shell
$ miniecs login
```

### Execute Sub-Command

A sub-command to exec command on container by specifying any resources.

```shell
$ miniecs exec \
    --cluster <cluster-name> \
    --service <service-name> \
    --container <container-name> \
    --command <command>
```

#### Options

| Option      | Explanation          | Required |
|-------------|----------------------|----------|
| --cluster   | ECS Cluster Name     | YES      |
| --service   | ECS Service Name     | YES      |
| --container | Container Name       | YES      |
| --command   | Command              | YES      |

### List Sub-Command

A sub-command to get table information of ecs cluster(s) and service(s).

```shell
$ miniecs list
```

## Reference

This code's internal pkg is based on the aws copilot-cli code (Apache License 2.0)

https://github.com/aws/copilot-cli

## License

[Apache License 2.0](https://github.com/jedipunkz/awscreds/blob/main/LICENSE)

## Author

[jedipunkz](https://twitter.com/jedipunkz)
