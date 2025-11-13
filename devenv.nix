{ pkgs, lib, config, inputs, ... }:

{
  packages = with pkgs; [ go golangci-lint ];
}
