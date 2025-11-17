{ pkgs, lib, config, inputs, ... }:

{
  packages = with pkgs; [ go golangci-lint kind kubernetes-helm ];

  tasks = { "test:unit" = { exec = "${pkgs.go}/bin/go test ./... -short"; }; };
}
