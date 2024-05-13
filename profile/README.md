# Open Component Model Community

Welcome to the Open Component Model community!

This repository outlines all the necessary steps to get started with learning about, using, and contributing to the OCM projects.
You can find all this and much, much more also on our [web page](https://ocm.software).

## What is the Open Component Model?

The Open Component Model provides a standard for describing delivery artifacts that can be accessed from many types of component repositories.

The following projects build the foundation of OCM:
- [OCM Specification](https://github.com/open-component-model/ocm-spec/blob/main/README.md) - The `ocm-spec` repository contains the OCM specification, which provides a formal description of OCM and its format to describe software artifacts and a storage layer to persist those and make them accessible from remote.
- [OCM Core Library](https://github.com/open-component-model/ocm#ocm-library) - The `ocm` core library is written in Golang and contains an API for interacting with OCM elements. A guided tour how to work with the library can be found [here](https://github.com/open-component-model/ocm/tree/main/examples/lib/tour#readme).
- [OCM CLI](https://github.com/open-component-model/ocm#ocm-cli) - With the `ocm` command line interface end users can interact with OCM elements. It makes it easy to create component versions and embed them in CI and CD processes. Examples can be found in [this Makefile](https://github.com/open-component-model/ocm/blob/main/examples/make/Makefile).
- [OCM Controller](https://github.com/open-component-model/ocm-controller) - The `ocm-controllers` are designed to enable the automated deployment of software using the [Open Component Model](https://ocm.software) and Flux.
- [OCM Website](https://github.com/open-component-model/ocm-website) - The `ocm-website` you are currently visiting. It is built using Hugo and hosted on Netlify.

Here are some suggested starting points:
- Read about the [problem statement](https://github.com/open-component-model/ocm-spec/tree/main/doc/introduction) that the OCM set of solutions can help to solve.
- Start with the documentation about [Model Elements](https://github.com/open-component-model/ocm-spec/blob/main/doc/01-model/02-elements-toplevel.md#model-elements).
- Check out this [demo](https://github.com/open-component-model/demo-secure-delivery) that shows an end-2-end scenario in an air-gapped environment, integrating OCM with [Flux](https://github.com/fluxcd/flux2).

## Contributing

We welcome all contributions from the community!

Please read the [Contributing Guide](https://github.com/open-component-model/community/tree/main/CONTRIBUTING.md) for instructions on how to contribute.
