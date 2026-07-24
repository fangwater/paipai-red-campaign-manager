module.exports = {
  apps: [
    {
      name: "paipai-xhs-jg-authd",
      script: "./bin/xhs-jg-authd",
      args: "serve",
      cwd: __dirname,
      interpreter: "none",
      exec_mode: "fork",
      instances: 1,
      autorestart: true,
      restart_delay: 5000,
      min_uptime: "10s",
      max_restarts: 20,
      kill_timeout: 30000,
      time: true,
      watch: false
    }
  ]
};
