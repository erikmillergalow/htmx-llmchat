// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::{
  api::process::Command,
  Manager,
};

fn main() {
  tauri::Builder::default()
    .setup(|app| {
        let _window = app.get_window("main").unwrap();
        tauri::async_runtime::spawn(async move {
            let (mut _rx, mut _child) = Command::new_sidecar("main")
                .expect("failed to setup main sidecar")
                .args(["serve"])
                .spawn()
                .expect("failed to spawn packaged pocketbase");
        });
        Ok(())
    })
    .run(tauri::generate_context!())
    .expect("error while running tauri application");
}
