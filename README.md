# 🌐 Website Blocker: Boost Your Productivity

Stay focused and eliminate distractions with this simple tool. Block websites that hinder your productivity and create a disciplined digital environment.


## ✨ Features

-  Site Access Scheduling 📅
-  Encrypted Password Login 🔒
-  Autonomous and Granular Site Blocking 🛑
-  Persistent Blocking Even After Restarts ♻️
-  Runs Seamlessly in the Background 🚀


## 🚨 Prerequisites

Before you begin:

- **Backup your `/etc/hosts` file** to avoid losing important configurations.

## 🛠️ How It Works

- Information for blocked sites and schedules are stored in yaml in configs folder
- The tool modifies the `/etc/hosts` file to block specified websites based on the yaml configs
- Websites are redirected to `localhost`, preventing them from loading via the local DNS server.
- Background runtime writes to nohup.out for debugging 

## 📖 Instructions

### 1. Backup Your `/etc/hosts` File

Save a copy of the file to ensure you can revert changes if needed.

### 2. Create service file in etc/systemd/system (optional)

To have persistence blocking on startup, create service file in **etc/systemd/system**

- Create service file
```
sudo nano /etc/systemd/system/selfcontrol.service
```
- Add configurations to file
```
[Unit]
Description=Selfcontrol website blocker

[Service]
ExecStart=<path_to_selfcontrol_application>
Environment="SELFCONTROL_STARTUP=1"

[Install]
WantedBy=multi-user.target
```

- Reload systemd

```
sudo systemctl daemon-reload
```

- Enable the service
  
```
sudo systemctl enable selfcontrol.service
```

- Start the service

```
sudo systemctl start selfcontrol.service
```
**OR**

- Restart the service if already started
```
sudo systemctl restart selfcontrol.service
```

- Check the Service Status
```
sudo systemctl status selfcontrol.service
```
### 3. Run the Application

Execute the tool with administrative privileges to apply the website blocks.

```
go build -o selfcontrol
sudo ./selfcontrol
```

## ⚠️ Disclaimer

- Editing the `/etc/hosts` file requires **administrative privileges**.
- Do not tamper with yaml configs directly. Instead use the CLI menu to interact with configs
- Use this tool responsibly and proceed with caution.

## 🚀 Let's Get Productive!

Take control of your time and focus!
