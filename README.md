## Tools for OS image inspection

These tools help to inspect OS images, mainly to debug reproducibility issues in the image build process. Currently only images with GPT/UKI are supported.

![overview](./overview.svg)


### Installation

```sh
go install github.com/katexochen/image-tools/{initrdsep,erofs-tool,uki-form-efi,unpart,unukify}
```
