export type XrayPreset = 'xhttp-auto' | 'tcp-vision' | 'xhttp-stream-one'
export const xrayPresetOptions = [
  { title: 'Reality XHTTP · auto · 2088', value: 'xhttp-auto' },
  { title: 'Reality TCP Vision · 10443', value: 'tcp-vision' },
  { title: 'Reality XHTTP · stream-one · 2089', value: 'xhttp-stream-one' },
]

// No shared credentials, and no unsolicited public listener.
export const createXrayDefaults = (preset: XrayPreset = 'xhttp-auto') => {
  const transport: 'tcp' | 'xhttp' = preset === 'tcp-vision' ? 'tcp' : 'xhttp'
  const port = preset === 'tcp-vision' ? 10443 : preset === 'xhttp-stream-one' ? 2089 : 2088
  return {
    enable: false, tag: `vless-reality-${preset}-${port}`, listen: '0.0.0.0', port,
    serverName: 'go.microsoft.com', dest: 'go.microsoft.com:443',
    privateKey: '', publicKey: '', shortId: '', fingerprint: 'chrome', transport,
    path: transport === 'tcp' ? '' : preset === 'xhttp-stream-one' ? '/xhttp-stream' : '/xhttp',
    mode: transport === 'tcp' ? '' : preset === 'xhttp-stream-one' ? 'stream-one' : 'auto',
  }
}