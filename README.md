# XRIPEP (X-Request-Id Envoy Plugin)

![GitHub Release](https://img.shields.io/github/v/release/achetronic/tnep)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/achetronic/tnep)
[![Go Report Card](https://goreportcard.com/badge/github.com/achetronic/tnep)](https://goreportcard.com/report/github.com/achetronic/tnep)
![GitHub License](https://img.shields.io/github/license/achetronic/tnep)

![GitHub User's stars](https://img.shields.io/github/stars/achetronic?label=Achetronic%20Stars)
![GitHub followers](https://img.shields.io/github/followers/achetronic?label=Achetronic%20Followers)

> [!IMPORTANT]  
> This is taking advantage of Go 1.24+ and new `go:wasmexport` directive, so using upstream Go.
> Thanks to [proxy-wasm community](https://github.com/proxy-wasm/proxy-wasm-go-sdk/tree/main)

## Description

Envoy WASM plugin to process X-Request-Id header (or the name you decide).
It's able to create a value, overwrite it or populate one that is already present.

## Motivation

In some environments, x-request-id is not generated or populated, and it's always useful to have it to allow you tracing
your customer's request better across your services. Some products like GCP Application Load Balancers are printing logs
without those IDs, and not event generating them by default. It's possible to modify this behavior using [Service Extensions](https://cloud.google.com/service-extensions/docs/overview)
that are Envoy plugins under the hood using [Proxy-Wasm](https://github.com/proxy-wasm) project. If you feel identified with this issue, this is the
plugin you are looking for.

## How to deploy

Deploying process for this plugin depends on the target (Istio or pure Envoy). You can find examples for both of them
in [documentation directory](./docs/samples). In fact, these examples are used by us to test, so you can rely on them.

Additionally, this plugin can be deployed as [GCP Service Extension](https://cloud.google.com/service-extensions/docs/overview) using
our OCI image

## How to develop

This plugin is developed using Go.

It's only needed to craft your code and execute the following command:

```console
make build run
```

## How releases are created

Each release of this plugin is completely automated by using [Github Actions' workflows](./github). 
Inside those workflows, we use recipes present at Makefile as much as possible to be completely transparent 
in the process we follow for building this.

Assets belonging to each version can be found attached to the corresponding release. OCI images are not yet published
until the whole process is well tested.


## How to collaborate

We are open to external collaborations for this project. For doing it you must:
- Open an issue explaining the problem
- Fork the repository 
- Make your changes to the code
- Open a PR 

> We are developers and hate bad code. For that reason we ask you the highest quality on each line of code to improve
> this project on each iteration. The code will always be reviewed and tested

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Special mention

This project was done using IDEs from JetBrains. They helped us to develop faster, so we recommend them a lot! 🤓

<img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jb_beam.png" alt="JetBrains Logo (Main) logo." width="150">