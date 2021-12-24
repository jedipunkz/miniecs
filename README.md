# miniecs

miniecs is a CLI tool for AWS ECS.

[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/jedipunkz/miniecs/Go-CI?style=flat-square)](https://github.com/jedipunkz/miniecs/actions?query=workflow%3AGo-CI)


## Requirement

- go 1.17.x or later

## Installation

```shell
go install github.com/jedipunkz/miniecs@latest
```

## Usage

```shell
$ miniecs exec --cluster <cluster-name> --container <container-name> --command <command>
```

### Options

| Option      | Explanation      | Required |
|-------------|------------------|----------|
| --cluster   | ECS Cluster Name | YES      |
| --container | Container Name   | YES      |
| --command   | Command          | YES      |

## License

[Apache License 2.0](https://github.com/jedipunkz/awscreds/blob/main/LICENSE)

## Author

[jedipunkz](https://twitter.com/jedipunkz)