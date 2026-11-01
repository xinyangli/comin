{
  self,
  nixpkgs,
  home-manager,
  treefmt-nix,
  flake-compat,
  system,
}:
let
  pkgs = nixpkgs.legacyPackages.${system};
  configurationName = "alice@vm";
  hostname = "different-host";
  repoPath = "/home/alice/comin-config";

  homeConfiguration = pkgs.writeText "comin-home-configuration.nix" ''
    { pkgs, cominMarker, ... }:
    {
      home.username = "alice";
      home.homeDirectory = "/home/alice";
      home.stateVersion = "25.11";
      home.file."comin-e2e-marker".text = cominMarker;
      nix.package = pkgs.nix;

      services.comin = {
        enable = true;
        hostname = "${hostname}";
        homeManagerConfiguration = "${configurationName}";
        remotes = [
          {
            name = "local";
            url = "${repoPath}";
            poller.period = 1;
          }
        ];
      };
    }
  '';

  initialConfiguration = home-manager.lib.homeManagerConfiguration {
    inherit pkgs;
    modules = [
      self.homeManagerModules.comin
      "${homeConfiguration}"
      { _module.args.cominMarker = "one"; }
    ];
  };

  updatedConfiguration = home-manager.lib.homeManagerConfiguration {
    inherit pkgs;
    modules = [
      self.homeManagerModules.comin
      "${homeConfiguration}"
      { _module.args.cominMarker = "two"; }
    ];
  };

  repoFixture =
    pkgs.runCommand "comin-home-manager-repository"
      {
        nativeBuildInputs = [
          pkgs.git
          pkgs.nix
        ];
      }
      ''
        mkdir -p "$out"
        cd "$out"
        cat > flake.nix <<'EOF'
        {
          inputs = {
            nixpkgs.url = "path:${nixpkgs}";
            home-manager.url = "path:${home-manager}";
            home-manager.inputs.nixpkgs.follows = "nixpkgs";
            comin.url = "path:${self}";
            comin.inputs.nixpkgs.follows = "nixpkgs";
            comin.inputs.home-manager.follows = "home-manager";
            comin.inputs.treefmt-nix.follows = "treefmt-nix";
            comin.inputs.flake-compat.follows = "flake-compat";
            treefmt-nix.url = "path:${treefmt-nix}";
            treefmt-nix.inputs.nixpkgs.follows = "nixpkgs";
            flake-compat.url = "path:${flake-compat}";
            flake-compat.flake = false;
          };

          outputs = { nixpkgs, home-manager, comin, ... }: {
            homeConfigurations."${configurationName}" = home-manager.lib.homeManagerConfiguration {
              pkgs = nixpkgs.legacyPackages.${system};
              modules = [
                comin.homeManagerModules.comin
                ./home-configuration.nix
                { _module.args.cominMarker = builtins.readFile ./comin-e2e-marker; }
              ];
            };
          };
        }
        EOF
        cp ${homeConfiguration} home-configuration.nix
        printf 'one' > comin-e2e-marker

        export HOME="$TMPDIR/home"
        mkdir -p "$HOME"
        nix --extra-experimental-features 'nix-command flakes' flake lock
        git init -b main
        git config user.name 'Comin Test'
        git config user.email 'comin@example.invalid'
        git add flake.nix flake.lock home-configuration.nix comin-e2e-marker
        git commit -m 'commit 1'
      '';
in
pkgs.testers.nixosTest {
  name = "comin-home-manager";

  nodes.machine =
    { lib, ... }:
    {
      users.users.alice = {
        isNormalUser = true;
        uid = 1000;
        home = "/home/alice";
        createHome = true;
      };
      environment.systemPackages = [ pkgs.git ];
      environment.etc."comin/initial-generation".source = initialConfiguration.activationPackage;
      nix.settings = {
        experimental-features = [
          "nix-command"
          "flakes"
        ];
        substituters = lib.mkForce [ ];
      };
      system.stateVersion = "25.11";
      virtualisation.useNixStoreImage = true;
      virtualisation.writableStore = true;
      virtualisation.writableStoreUseTmpfs = false;
      virtualisation.diskSize = 8192;
      virtualisation.additionalPaths = [
        updatedConfiguration.activationPackage.stdenv
        updatedConfiguration.config.home-files
        updatedConfiguration.config.home.path
      ];
    };

  testScript = ''
    import json

    machine.wait_for_unit("multi-user.target")
    machine.succeed("cp -a ${repoFixture}/. ${repoPath}")
    machine.succeed("chmod -R u+w ${repoPath}")
    machine.succeed("chown -R alice:users ${repoPath}")
    machine.succeed("loginctl enable-linger alice")
    machine.wait_for_unit("user@1000.service")

    machine.succeed("su - alice -c 'XDG_RUNTIME_DIR=/run/user/1000 /etc/comin/initial-generation/activate'")
    machine.succeed("su - alice -c 'XDG_RUNTIME_DIR=/run/user/1000 systemctl --user start comin.service'")
    machine.succeed("test \"$(cat /home/alice/comin-e2e-marker)\" = one")
    machine.succeed("su - alice -c 'XDG_RUNTIME_DIR=/run/user/1000 systemctl --user is-active comin.service'")
    commit1 = machine.succeed("su - alice -c 'git -C ${repoPath} rev-parse HEAD'").strip()
    machine.wait_until_succeeds(f"grep -q '{commit1}' /home/alice/.local/share/comin/store.json", timeout=120)

    machine.succeed("su - alice -c 'cd ${repoPath} && printf two > comin-e2e-marker && git add comin-e2e-marker && git commit -m \"commit 2\"'")
    commit2 = machine.succeed("su - alice -c 'git -C ${repoPath} rev-parse HEAD'").strip()
    machine.wait_until_succeeds("test \"$(cat /home/alice/comin-e2e-marker)\" = two", timeout=120)

    machine.succeed("su - alice -c 'XDG_RUNTIME_DIR=/run/user/1000 systemctl --user is-active comin.service'")
    logs = machine.succeed("su - alice -c 'XDG_RUNTIME_DIR=/run/user/1000 journalctl --user -u comin.service --no-pager'")
    assert commit2 in logs
    assert "/activate" in logs

    store = json.loads(machine.succeed("cat /home/alice/.local/share/comin/store.json"))
    deployments = store["deployments"]
    assert deployments
    latest = deployments[0]
    git = latest["generation"]["source"]["git"]
    selected_commit = git.get("selected_commit_id", git.get("selectedCommitId"))
    assert selected_commit == commit2
    assert latest.get("profile_path", latest.get("profilePath", "")) == ""
    restart_comin = latest.get("restart_comin", latest.get("restartComin", False))
    assert restart_comin is False
    for deployment in deployments:
        generation = deployment["generation"]
        assert generation.get("out_path", generation.get("outPath", "")) != ""
  '';
}
