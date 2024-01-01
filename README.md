<a name="readme-top"></a>

<!--
*** Thanks for checking out the Gate Plugin Template. If you have a suggestion
*** that would make this better, please fork the repo and create a pull request
*** or simply open an issue with the tag "enhancement".
*** Don't forget to give the project a star!
*** Thanks again! Now go create something AMAZING! :D
-->

[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![License][license-shield]][license-url]

<br />
<div align="center">
  <a href="https://github.com/minekube/gate-plugin-template">
    <img src="https://raw.githubusercontent.com/minekube/gate-plugin-template/main/assets/hero.png" alt="Logo" width="128" height="128">
  </a>

<h3 align="center">Gate Starter Plugin Template</h3>

  <p align="center">
    An awesome template for creating your Minecraft proxy powered by Minekube Gate!
    <br />
    <br />
    <a href="https://gate.minekube.com/developers/"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://minekube.com/discord">Discord</a>
    ·
    <a href="https://github.com/minekube/gate/issues">Report Bug</a>
    ·
    <a href="https://github.com/minekube/gate/issues">Request Feature</a>
  </p>
</div>

## About The Project

[![Product Name Screen Shot][product-screenshot]](https://gate.minekube.com)

This template repository bootstraps your [Minekube Gate](https://github.com/minekube/gate) project, a customizable
Minecraft proxy written in Go.

## What's Included?

- `gate.go`: The main entry point of the application.
- `plugins`: The directory for your custom plugins.
- `config.yml`: A minimal Gate configuration file.
- `Dockerfile`: A Dockerfile for building a Docker image.
- `.github/workflows`: GitHub Action for testing, linting, releasing on tags and publishing Docker images to ghcr.io.
- `Makefile`: Contains commands for testing and linting.
- `renovate.json`: Configuration file for Renovate automatic dependency updates.

<details>
<summary><strong>Prerequisites</strong></summary>

## Prerequisites

- [Go](https://golang.org/doc/install) - The Go Programming Language
- [Git](https://git-scm.com/downloads) - Distributed Version Control System
- [GoLand](https://www.jetbrains.com/go/) / [VSCode](https://code.visualstudio.com/) - Gophers' favorite IDEs

</details>

## Getting Started

1. Fork this repository on GitHub.
2. Clone forked repository (`git clone <your-forked-repo-url>`)
3. Open project in your favorite Go IDE.
4. Run the proxy: `go run .`
5. Start customizing Gate to your needs!

## Usage

To create a new Gate plugin, follow these steps:

1. Create and write your plugin code in a new `plugins/xyz/xyz.go` file.
2. Add your exported plugin to the `proxy.Plugins` slice in `gate.go`.
3. Build and run Gate with: `go run .`

Use the `-d` flag to run Gate in debug mode if you encounter issues. (`go run . -d`)

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any
contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feat/AmazingFeature`)
5. Open a Pull Request on GitHub

<p align="right">(<a href="#readme-top">back to top</a>)</p>


[contributors-shield]: https://img.shields.io/github/contributors/minekube/gate.svg?style=for-the-badge

[contributors-url]: https://github.com/minekube/gate/graphs/contributors

[forks-shield]: https://img.shields.io/github/forks/minekube/gate-plugin-template.svg?style=for-the-badge

[forks-url]: https://github.com/minekube/gate-plugin-template/network/members

[stars-shield]: https://img.shields.io/github/stars/minekube/gate.svg?style=for-the-badge

[stars-url]: https://github.com/minekube/gate-plugin-template/stargazers

[issues-shield]: https://img.shields.io/github/issues/minekube/gate.svg?style=for-the-badge

[issues-url]: https://github.com/minekube/gate-plugin-template/issues

[license-shield]: https://img.shields.io/github/license/minekube/gate.svg?style=for-the-badge

[license-url]: https://github.com/minekube/gate/blob/master/LICENSE

[product-screenshot]: https://github.com/minekube/gate/raw/master/.web/docs/public/og-image.png