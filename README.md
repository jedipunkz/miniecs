# miniecs

miniecs is a CLI tool for AWS ECS.


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
