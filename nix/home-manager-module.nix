{ self }:
{
  config,
  pkgs,
  lib,
  ...
}:
let
  cfg = config;
  stateDir = "${config.xdg.dataHome}/comin";
  cominConfigLib = import ./comin-config.nix {
    inherit
      config
      pkgs
      lib
      stateDir
      ;
  };
  inherit (cominConfigLib) cominConfigYaml;

  inherit (pkgs.stdenv.hostPlatform) system;
  inherit (cfg.services.comin) package;
in
{
  imports = [ ./module-options.nix ];
  config = lib.mkIf cfg.services.comin.enable {
    assertions = [
      {
        assertion = package != null;
        message = "`services.comin.package` cannot be null.";
      }
      {
        assertion = package == null -> lib.elem system (lib.attrNames self.packages);
        message = "comin: ${system} is not supported by the Flake.";
      }
      {
        assertion = cfg.services.comin.homeManagerConfiguration != "";
        message = "services.comin.homeManagerConfiguration must not be empty for Home Manager deployments";
      }
      {
        assertion = pkgs.stdenv.hostPlatform.isLinux;
        message = "comin Home Manager deployments currently support only Linux systemd user services";
      }
      {
        assertion = cfg.services.comin.repositoryType == "flake";
        message = "comin Home Manager deployments support only the flake repository type";
      }
      {
        assertion = lib.all (
          remote: remote.branches.main.operation != "boot" && remote.branches.testing.operation != "boot"
        ) cfg.services.comin.remotes;
        message = "comin Home Manager deployments do not support the boot operation";
      }
    ];

    services.comin.package = lib.mkDefault (pkgs.comin or self.packages.${system}.comin or null);
    services.comin.homeManagerConfiguration = lib.mkDefault config.home.username;
    services.comin.hostname = lib.mkDefault config.home.username;
    home.packages = [ package ];

    systemd.user.services.comin = {
      Install.WantedBy = [ "default.target" ];
      Service = {
        ExecStart =
          (lib.getExe package)
          + (lib.optionalString cfg.services.comin.debug " --debug ")
          + " run --home-manager --config ${cominConfigYaml}";
        Restart = "always";
        Environment = [
          "PATH=${
            lib.makeBinPath [
              config.nix.package
              pkgs.git
              pkgs.openssh
            ]
          }"
          "CURL_CA_BUNDLE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
        ];
        "X-SwitchMethod" = "keep-old";
      };
    };
  };
}
