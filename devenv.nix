{ pkgs, lib, config, inputs, ... }:

{
  packages = with pkgs; [ go golangci-lint kind kubernetes-helm ];
}
