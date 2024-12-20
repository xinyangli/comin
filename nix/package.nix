{
  lib,
  buildGoModule,
  git,
  makeWrapper,
  writeTextFile,
}:

let
  # - safe.directory: this is to allow comin to fetch local repositories belonging
  #   to other users. Otherwise, comin fails with:
  #   Pull from remote 'local' failed: unknown error: fatal: detected dubious ownership in repository
  # - core.hooksPath: to avoid Git executing hooks from a repository belonging to another user
  gitConfigFile = writeTextFile {
    name = "git.config";
    text = ''
      [safe]
         directory = *
      [core]
         hooksPath = /dev/null
    '';
  };
in

buildGoModule rec {
  pname = "comin";
  version = "0.6.0";
  nativeCheckInputs = [ git ];
  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../cmd
      ../internal
      ../go.mod
      ../go.sum
      ../main.go
    ];
  };
  vendorHash = "sha256-4piLsTi/6A82bmWp/m6S32DZ95V2Pjzx5Cb12jMJTRM=";
  ldflags = [
    "-X github.com/nlewo/comin/cmd.version=${version}"
  ];
  buildInputs = [ makeWrapper ];
  postInstall = ''
    # This is because Nix needs Git at runtime by the go-git library
    wrapProgram $out/bin/comin --set GIT_CONFIG_SYSTEM ${gitConfigFile} --prefix PATH : ${git}/bin
  '';

  meta = {
    mainProgram = "comin";
  };
}
