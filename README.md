# WhatGrowsHere Backend Stack

This is the new WhatGrowsHere Backend Stack, rewritten in Go, using MongoDB.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [API documentation](#api documentation)
- [Support](#support)
- [Contributing](#contributing)

## Prerequisites

- Go version > 1.7
- Go environment set up
- [govendor installed](https://github.com/kardianos/govendor)
- MongoDB version > 3.4.4 (however, keep in mind that ^3.4 will be deprecated soon - 3.6.0 offers new improvements which we will harness soon)

## Installation

1. Download the data files

You will need to have the [gdrive CLI tool](https://github.com/prasmussen/gdrive) installed and in your PATH

```sh
cd data
bash download.sh
```

2. Import the data files into Mongo.

This step could be better parallelized, as it can take quite a while (~8h on a mid-range laptop), however since you should only run this once, it is not a priority.

```sh
cd import
govendor sync
go build import.go
./import
```

3. Test the webserver
```sh
cd api
govendor sync
go build index.go
./index
```

## Usage

./index is the main API server, powered by Gin.

Set the `PORT` env-var to change the port it listens on.

A good idea is to have nginx in front for reverse-proxying (this also enables multiple instances in case of a huge workload).


## API documentation

TODO

## Support

Please [open an issue](https://github.com/growingdatafoundation/wgh-backend/issues/new) for support.

## Contributing

Please contribute using [Github Flow](https://guides.github.com/introduction/flow/). Create a branch, add commits, and [open a pull request](https://github.com/growingdatafoundation/wgh-backend/compare).