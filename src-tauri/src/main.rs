// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use tauri::{
  api::process::{Command, CommandEvent},
  Manager,
};

fn main() {
  tauri::Builder::default()
    .setup(|app| {
        let _window = app.get_window("main").unwrap();
        tauri::async_runtime::spawn(async move {
            let (mut rx, mut _child) = Command::new_sidecar("main")
                .expect("failed to setup main sidecar")
                .args(["serve"])
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
