// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::{
  api::process::{Command, CommandEvent},
  Manager,
};
use std::path::PathBuf;
use std::sync::Arc;

#[tauri::command]
fn open_link(window: tauri::Window, url: String) -> Result<(), String> {
    tauri::api::shell::open(&window.shell_scope(), url, None)
        .map_err(|e| e.to_string())
}

fn main() {
  tauri::Builder::default()
    .setup(|app| {
        let window = app.get_window("main").unwrap();

        // inject JavaScript to handle link clicks
        window.eval(r#"
            document.addEventListener('click', (e) => {
                const target = e.target.closest('a');
                if (target && target.getAttribute('target') === '_blank') {
                    e.preventDefault();
                    window.__TAURI__.invoke('open_link', { url: target.href });
                }
            });
        "#).expect("Failed to inject JavaScript");


        let pb_data_path: PathBuf = tauri::api::path::app_data_dir(&Default::default())
            .expect("failed to resolve app data directory")
            .join("htmx_llmchat_db");

        std::fs::create_dir_all(&pb_data_path).expect("Failed to create data directory");

        let pb_data_path_str = pb_data_path.to_string_lossy().into_owned();

        let data_path_str = Arc::new(pb_data_path_str); 

        tauri::async_runtime::spawn(async move {
            let pb_data_path = Arc::clone(&data_path_str);
            let (mut rx, mut _child) = Command::new_sidecar("main")
                .expect("failed to setup main sidecar")
                .args(["serve", "--dir", &pb_data_path])
                // .args(["serve", "-http='127.0.0.1:3000"])
                .spawn()
                .expect("failed to spawn packaged pocketbase");
            
            // Now handle the CommandEvents from the receiver
            tauri::async_runtime::spawn(async move {
                while let Some(event) = rx.recv().await {
                    match event {
                        CommandEvent::Stdout(line) => {
                            println!("Sidecar stdout: {}", line);
                        }
                        CommandEvent::Stderr(line) => {
                            eprintln!("Sidecar stderr: {}", line);
                        }
                        _ => {}
                    }
                }
            });
        });
        
        Ok(())
    })
    .run(tauri::generate_context!())
    .expect("error while running tauri application");
}
