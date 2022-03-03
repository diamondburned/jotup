self: super: {
	go = super.go.overrideAttrs (old: {
		version = "1.17.6";
		src = builtins.fetchurl {
			url    = "https://go.dev/dl/go1.17.6.src.tar.gz";
			sha256 = "sha256:1j288zwnws3p2iv7r938c89706hmi1nmwd8r5gzw3w31zzrvphad";
		};
		doCheck = false;
		patches = [
			# cmd/go/internal/work: concurrent ccompile routines
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/4e07fa9fe4e905d89c725baed404ae43e03eb08e.patch")
			# cmd/cgo: concurrent file generation
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/432db23601eeb941cf2ae3a539a62e6f7c11ed06.patch")
		];
	});
	buildGoModule = super.buildGoModule.override {
		inherit (self) go;
	};
	gotools = super.gotools; # TODO

	lasem = super.lasem.overrideAttrs (old: {
		version = "0.7.0-ee047b6";

		src = super.fetchFromGitHub {
			owner  = "mjakeman";
			repo   = "lasem";
			rev    = "94e2b6c";
			sha256 = "0iqh0kqch4ds1jvxm2rkbgl08hd4ssr3s4l7d7m1zaf1g0jkmdz4";
		};

		buildInputs = (old.buildInputs or []) ++ (with super; [
			bison
			flex
		]);

		nativeBuildInputs = with super; [
			pkg-config
			intltool
			meson
			ninja
			gi-docgen
			gobject-introspection
		];

		# We have failing tests.
		doCheck = false;

		# We don't need these.
		mesonFlags = [
			"-Ddocs=disabled"
			"-Ddemo=disabled"
		];

		outputs = [ "out" "dev" "man" ];

		# po/meson.build:2:5: ERROR: Tried to create target "lasem-0.7-it.mo", but a target of that name already exists.
		preConfigure = (old.preConfigure or "") + ''
			rm po/*.po
			:> po/LINGUAS
		'';
	});
}
