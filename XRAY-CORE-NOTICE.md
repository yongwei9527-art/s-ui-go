# Bundled Xray-core notice

The Linux amd64 release package includes the `xray` executable from
[XTLS/Xray-core](https://github.com/XTLS/Xray-core).

- Upstream version: `v26.3.27`
- Upstream asset: `Xray-linux-64.zip`
- Upstream SHA-256: `23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae`
- Source and license: <https://github.com/XTLS/Xray-core>
- Upstream license: Mozilla Public License 2.0 (MPL-2.0)

The executable is an independently maintained upstream component. Its source,
license, security notices, and upgrade cadence are provided by XTLS/Xray-core.
S-UI starts it only for the XHTTP/Reality sidecar when `SUI_XRAY_BINARY` points
to the bundled executable.
