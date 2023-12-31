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
[![MIT License][license-shield]][license-url]

<br />
<div align="center">
  <a href="https://github.com/minekube/gate-plugin-template">
    <img src="https://github.com/minekube/gate-plugin-template/blob/main/assets/hero.png" alt="Logo" width="80" height="80">
  </a>

<h3 align="center">Gate Plugin Template</h3>

  <p align="center">
    An awesome template for creating plugins for Minekube Gate!
    <br />
    <a href="https://gate.minekube.com/developers/"><strong>Explore the docs »</strong></a>
    <br />
    <br />
    <a href="https://github.com/minekube/gate-plugin-template">View Demo</a>
    ·
    <a href="https://github.com/minekube/gate/issues">Report Bug</a>
    ·
    <a href="https://github.com/minekube/gate/issues">Request Feature</a>
  </p>
</div>

<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#installation">Installation</a></li>
      </ul>
    </li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#contact">Contact</a></li>
  </ol>
</details>

## About The Project

[![Product Name Screen Shot][product-screenshot]](https://example.com)

This is a template repository for creating plugins for [Minekube Gate](https://github.com/minekube/gate), a scalable
Minecraft proxy written in Go.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Getting Started

1. Clone this repository: `git clone https://github.com/minekube/gate-plugin-template.git`
2. Install dependencies: `go get`
3. Run the project: `go run .`

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Usage

To create a new Gate plugin, follow these steps:

1. Create a new package in the `plugins` directory.
2. Implement your plugin's functionality in the `plugins/xyz/xyz.go` file.
3. Add your plugin to the `proxy.Plugins` slice in `gate.go`.
4. Build and run Gate with: `go run .`

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any
contributions you make are **greatly appreciated**.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## License

Distributed under the MIT License. See `LICENSE.txt` for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>

## Contact

Your Name - [@your_twitter](https://twitter.com/your_username) - email@example.com

Project Link: [https://github.com/minekube/gate-plugin-template](https://github.com/minekube/gate-plugin-template)

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