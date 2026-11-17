package object

// Version is the version of Goblin. It lives here rather than in package
// main because compiled programs link object and not main, and both the CLI
// (`goblin --version`) and the http module's User-Agent read it.
const Version = "0.1.0"
