use std::fs::{self, OpenOptions};
use std::os::windows::process::CommandExt;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use serde::Deserialize;
use tauri::{
    menu::{Menu, MenuItem, PredefinedMenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager, Url, WebviewUrl, WebviewWindowBuilder,
};

const CREATE_NO_WINDOW: u32 = 0x08000000;
const INJECT_SCRIPT: &str = include_str!("../../ui/inject.js");

#[derive(Clone)]
pub struct AppState {
    pub backend: Arc<Mutex<Option<Child>>>,
    pub base_dir: PathBuf,
    pub exe_path: PathBuf,
    pub log_path: PathBuf,
    pub version_path: PathBuf,
}

#[derive(Debug, Deserialize)]
struct GithubRelease {
    tag_name: String,
    assets: Vec<GithubAsset>,
}

#[derive(Debug, Deserialize)]
struct GithubAsset {
    name: String,
    browser_download_url: String,
}

impl AppState {
    pub fn new() -> Self {
        let current_exe = std::env::current_exe().unwrap_or_else(|_| PathBuf::from("."));
        let base_dir = current_exe
            .parent()
            .unwrap_or_else(|| Path::new("."))
            .to_path_buf();

        let exe_path = base_dir.join("wb2api.exe");
        let log_dir = base_dir.join("logs");
        let _ = fs::create_dir_all(&log_dir);
        let log_path = log_dir.join("wb2api.log");
        let version_path = base_dir.join("version.txt");

        if !version_path.exists() {
            let _ = fs::write(&version_path, "v1.11.1");
        }

        Self {
            backend: Arc::new(Mutex::new(None)),
            base_dir,
            exe_path,
            log_path,
            version_path,
        }
    }

    pub fn start_backend(&self) -> Result<(), String> {
        let mut lock = self.backend.lock().map_err(|e| e.to_string())?;
        if lock.is_some() {
            return Ok(());
        }

        if !self.exe_path.exists() {
            return Err(format!("未找到可执行文件: {:?}", self.exe_path));
        }

        let log_file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.log_path)
            .map_err(|e| format!("无法打开日志文件: {}", e))?;

        let err_file = log_file.try_clone().map_err(|e| e.to_string())?;

        let child = Command::new(&self.exe_path)
            .current_dir(&self.base_dir)
            .creation_flags(CREATE_NO_WINDOW)
            .stdout(Stdio::from(log_file))
            .stderr(Stdio::from(err_file))
            .spawn()
            .map_err(|e| format!("启动 wb2api 失败: {}", e))?;

        *lock = Some(child);
        Ok(())
    }

    pub fn stop_backend(&self) {
        if let Ok(mut lock) = self.backend.lock() {
            if let Some(mut child) = lock.take() {
                let _ = child.kill();
                let _ = child.wait();
            }
        }
    }

    pub fn restart_backend(&self) -> Result<(), String> {
        self.stop_backend();
        thread::sleep(Duration::from_millis(600));
        self.start_backend()
    }

    pub fn is_running(&self) -> bool {
        if let Ok(mut lock) = self.backend.lock() {
            if let Some(child) = lock.as_mut() {
                match child.try_wait() {
                    Ok(None) => return true,
                    _ => {
                        *lock = None;
                        return false;
                    }
                }
            }
        }
        false
    }

    pub fn get_current_version(&self) -> String {
        fs::read_to_string(&self.version_path)
            .unwrap_or_else(|_| "v1.11.1".to_string())
            .trim()
            .to_string()
    }
}

pub fn check_and_update(state: AppState) {
    thread::spawn(move || {
        let repos = [
            "HOnnTaka/workbuddy2api-desktop",
            "linguo2625469/workbuddy2api-panel",
        ];

        let client = match reqwest::blocking::Client::builder()
            .timeout(Duration::from_secs(15))
            .user_agent("WorkBuddy2API-Desktop/1.0")
            .build()
        {
            Ok(c) => c,
            Err(_) => return,
        };

        for repo in repos {
            let api_url = format!("https://api.github.com/repos/{}/releases/latest", repo);
            let release: GithubRelease = match client.get(&api_url).send().and_then(|r| r.json()) {
                Ok(data) => data,
                Err(_) => continue,
            };

            let current_ver = state.get_current_version();
            let latest_ver = release.tag_name.trim();

            if latest_ver != current_ver {
                if let Some(asset) = release
                    .assets
                    .iter()
                    .find(|a| a.name.contains("windows-amd64.zip"))
                {
                    let temp_zip = state.base_dir.join("_update_temp.zip");
                    let temp_dir = state.base_dir.join("_update_temp");

                    if let Ok(mut resp) = client.get(&asset.browser_download_url).send() {
                        if let Ok(mut file) = fs::File::create(&temp_zip) {
                            let _ = std::io::copy(&mut resp, &mut file);
                        }
                    }

                    if temp_zip.exists() {
                        let _ = fs::remove_dir_all(&temp_dir);
                        let _ = fs::create_dir_all(&temp_dir);

                        let unpack_res = Command::new("tar")
                            .args(["-xf", temp_zip.to_str().unwrap(), "-C", temp_dir.to_str().unwrap()])
                            .creation_flags(CREATE_NO_WINDOW)
                            .status();

                        if unpack_res.is_ok() {
                            state.stop_backend();
                            thread::sleep(Duration::from_millis(1000));

                            if let Ok(entries) = fs::read_dir(&temp_dir) {
                                for entry in entries.flatten() {
                                    let path = entry.path();
                                    if path.is_file() && path.file_name().unwrap_or_default() == "wb2api.exe" {
                                        let backup_exe = state.base_dir.join("wb2api.exe.bak");
                                        let _ = fs::remove_file(&backup_exe);
                                        let _ = fs::rename(&state.exe_path, &backup_exe);
                                        let _ = fs::copy(&path, &state.exe_path);
                                        let _ = fs::write(&state.version_path, latest_ver);
                                        break;
                                    } else if path.is_dir() {
                                        let sub_exe = path.join("wb2api.exe");
                                        if sub_exe.exists() {
                                            let backup_exe = state.base_dir.join("wb2api.exe.bak");
                                            let _ = fs::remove_file(&backup_exe);
                                            let _ = fs::rename(&state.exe_path, &backup_exe);
                                            let _ = fs::copy(&sub_exe, &state.exe_path);
                                            let _ = fs::write(&state.version_path, latest_ver);
                                            break;
                                        }
                                    }
                                }
                            }

                            let _ = fs::remove_file(&temp_zip);
                            let _ = fs::remove_dir_all(&temp_dir);
                            let _ = state.start_backend();
                        }
                    }
                    return;
                }
            }
        }
    });
}

pub fn run() {
    let state = AppState::new();
    let _ = state.start_backend();

    let update_state = state.clone();
    thread::spawn(move || {
        thread::sleep(Duration::from_secs(5));
        check_and_update(update_state);
    });

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| {
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.show();
                let _ = window.set_focus();
            }
        }))
        .manage(state.clone())
        .setup(move |app| {
            // 动态构建主窗口并注入全站样式与守护脚本
            let _window = WebviewWindowBuilder::new(
                app,
                "main",
                WebviewUrl::App(PathBuf::from("index.html")),
            )
            .title("WorkBuddy2API Panel")
            .inner_size(1120.0, 780.0)
            .min_inner_size(820.0, 600.0)
            .resizable(true)
            .initialization_script(INJECT_SCRIPT)
                        .on_navigation(|url| {
                if url.scheme() == "http" || url.scheme() == "https" {
                    if url.host_str() != Some("127.0.0.1") && url.host_str() != Some("localhost") {
                        let _ = open::that(url.as_str());
                        return false;
                    }
                }
                true
            })
.build()?;

            let app_handle = app.handle().clone();

            // 监听服务健康检查，就绪后无缝切入 Web 面板
            thread::spawn(move || {
                let client = match reqwest::blocking::Client::builder()
                    .timeout(Duration::from_millis(500))
                    .build()
                {
                    Ok(c) => c,
                    Err(_) => return,
                };

                for _ in 0..120 {
                    thread::sleep(Duration::from_millis(150));
                    let is_ready = if let Ok(resp) = client.get("http://127.0.0.1:7863/panel/").send() {
                        resp.status().is_success() || resp.status().as_u16() == 503
                    } else if let Ok(resp) = client.get("http://127.0.0.1:7863/healthz").send() {
                        resp.status().is_success() || resp.status().as_u16() == 503
                    } else {
                        false
                    };

                    if is_ready {
                        thread::sleep(Duration::from_millis(100));
                        if let Some(window) = app_handle.get_webview_window("main") {
                            if let Ok(url) = Url::parse("http://127.0.0.1:7863/panel/") {
                                let _ = window.navigate(url);
                            }
                        }
                        break;
                    }
                }
            });

            // 托盘右键菜单
            let open_item = MenuItem::with_id(app, "open", "打开主面板", true, None::<&str>)?;
            let browser_item = MenuItem::with_id(app, "browser", "在浏览器中打开 WebUI", true, None::<&str>)?;
            let sep1 = PredefinedMenuItem::separator(app)?;
            let restart_item = MenuItem::with_id(app, "restart", "重启核心服务", true, None::<&str>)?;
            let toggle_item = MenuItem::with_id(app, "toggle", "停止/启动服务", true, None::<&str>)?;
            let sep2 = PredefinedMenuItem::separator(app)?;
            let about_item = MenuItem::with_id(app, "about", "关于与版本信息", true, None::<&str>)?;
            let log_item = MenuItem::with_id(app, "logs", "查看运行日志", true, None::<&str>)?;
            let update_item = MenuItem::with_id(app, "update", "检查更新 (GitHub)", true, None::<&str>)?;
            let sep3 = PredefinedMenuItem::separator(app)?;
            let quit_item = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;

            let menu = Menu::with_items(
                app,
                &[
                    &open_item,
                    &browser_item,
                    &sep1,
                    &restart_item,
                    &toggle_item,
                    &sep2,
                    &about_item,
                    &log_item,
                    &update_item,
                    &sep3,
                    &quit_item,
                ],
            )?;

            let _tray = TrayIconBuilder::new()
                .icon(app.default_window_icon().cloned().expect("缺少应用图标"))
                .tooltip("WorkBuddy2API Panel")
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(move |app_handle, event| {
                    let st = app_handle.state::<AppState>();
                    match event.id().as_ref() {
                        "open" => {
                            if let Some(w) = app_handle.get_webview_window("main") {
                                let _ = w.show();
                                let _ = w.set_focus();
                            }
                        }
                        "browser" => {
                            let _ = open::that("http://127.0.0.1:7863/panel/");
                        }
                        "restart" => {
                            if let Some(w) = app_handle.get_webview_window("main") {
                                let _ = w.eval("if (window.__wb2api_trigger_restart_ui) window.__wb2api_trigger_restart_ui();");
                            }
                            let _ = st.restart_backend();
                        }
                        "toggle" => {
                            if st.is_running() {
                                st.stop_backend();
                            } else {
                                let _ = st.start_backend();
                            }
                        }
                        "about" => {
                            if let Some(w) = app_handle.get_webview_window("main") {
                                let _ = w.show();
                                let _ = w.set_focus();
                                let _ = w.eval("if (window.__wb2api_show_about) window.__wb2api_show_about();");
                            }
                        }
                        "logs" => {
                            let log_file = st.log_path.clone();
                            thread::spawn(move || {
                                let _ = Command::new("notepad.exe")
                                    .arg(log_file)
                                    .spawn();
                            });
                        }
                        "update" => {
                            check_and_update(st.inner().clone());
                        }
                        "quit" => {
                            st.stop_backend();
                            app_handle.exit(0);
                        }
                        _ => {}
                    }
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event
                    {
                        let app = tray.app_handle();
                        if let Some(window) = app.get_webview_window("main") {
                            let _ = if window.is_visible().unwrap_or(false) {
                                window.hide()
                            } else {
                                window.show().and_then(|_| window.set_focus())
                            };
                        }
                    }
                })
                .build(app)?;

            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .run(tauri::generate_context!())
        .expect("运行 Tauri 应用失败");
}
