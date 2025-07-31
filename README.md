# Git Inquisitor

## Description

A git repository analysis tool designed to provide teams with useful information about a repository and its contributors. It provides history details from the HEAD of the provided repository, file level contribution statistics (enhanced blame), and contributor level statistics similar to what is provided by GitHub.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Contributing](#contributing)
- [License](#license)

## Installation

You can install `git-inquisitor` using `go install`:

```bash
go install github.com/user/git-inquisitor-go/cmd/git-inquisitor@latest
```

Alternatively, you can build it from source:

```bash
git clone https://github.com/user/git-inquisitor-go.git
cd git-inquisitor-go
make build
```

## Usage

```
❯ ./git-inquisitor --help
Usage: git-inquisitor [OPTIONS] COMMAND [ARGS]...

Options:
  --help  Show this message and exit.

Commands:
  collect
  report
```

**Collecting repository information:**

```
❯ ./git-inquisitor collect --help
Usage: ./git-inquisitor collect [OPTIONS] REPO_PATH

Options:
  --help  Show this message and exit.
```

**Produce report against collected information:**

```
❯ ./git-inquisitor report --help
Usage: ./git-inquisitor report [OPTIONS] REPO_PATH {html|json}

Options:
  -o, --output-file-path TEXT  Output file path
  --help                       Show this message and exit.
```
