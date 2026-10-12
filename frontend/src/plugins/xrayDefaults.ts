// No static credentials: each new node obtains its own key pair when saved.
export const createXrayDefaults = () => ({
  enable: false, tag: 'vless-reality-xhttp-2088', listen: '0.0.0.0', port: 2088,
  serverName: 'go.microsoft.com', dest: 'go.microsoft.com:443',
  privateKey: '', publicKey: '', shortId: '', fingerprint: 'chrome',
  transport: 'xhttp' as const, path: '/xhttp', mode: 'auto',
})