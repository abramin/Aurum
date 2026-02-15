use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result};
use chrono::{Duration, Utc};
use rusqlite::{params, Connection};
use serde::Serialize;
use tauri::{AppHandle, Manager};

const DB_FILENAME: &str = "aurum.sqlite3";
const DEFAULT_ACCOUNT_NAME: &str = "Primary Checking";
const DEFAULT_ACCOUNT_TYPE: &str = "current";
const DEFAULT_ACCOUNT_BALANCE: f64 = 2_500.0;

const SCHEMA_SQL: &str = r#"
CREATE TABLE IF NOT EXISTS accounts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  balance REAL NOT NULL,
  is_liquid INTEGER NOT NULL CHECK (is_liquid IN (0, 1)),
  growth_rate_apr REAL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS transactions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id INTEGER NOT NULL,
  amount REAL NOT NULL,
  date TEXT NOT NULL,
  payee TEXT,
  category TEXT,
  linked_transfer_id INTEGER,
  FOREIGN KEY(account_id) REFERENCES accounts(id)
);

CREATE TABLE IF NOT EXISTS scheduled_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  account_id INTEGER NOT NULL,
  amount REAL NOT NULL,
  frequency TEXT NOT NULL,
  next_date TEXT NOT NULL,
  type TEXT NOT NULL,
  target_account_id INTEGER,
  FOREIGN KEY(account_id) REFERENCES accounts(id),
  FOREIGN KEY(target_account_id) REFERENCES accounts(id)
);

CREATE TABLE IF NOT EXISTS budgets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  category TEXT NOT NULL,
  monthly_limit REAL NOT NULL
);
"#;

#[derive(Debug, Clone, Serialize)]
struct ForecastPoint {
  date: String,
  balance: f64,
}

fn database_path(app: &AppHandle) -> Result<PathBuf> {
  let app_data_dir = app
    .path()
    .app_data_dir()
    .context("failed to resolve app data directory")?;

  fs::create_dir_all(&app_data_dir)
    .with_context(|| format!("failed creating app data directory: {}", app_data_dir.display()))?;

  Ok(app_data_dir.join(DB_FILENAME))
}

fn bootstrap_database_at(db_path: &Path) -> Result<()> {
  let conn = Connection::open(db_path)
    .with_context(|| format!("failed opening sqlite database at {}", db_path.display()))?;

  conn.execute_batch(SCHEMA_SQL)
    .context("failed creating schema")?;

  let account_count: i64 = conn
    .query_row("SELECT COUNT(1) FROM accounts", [], |row| row.get(0))
    .context("failed counting accounts")?;

  if account_count == 0 {
    conn.execute(
      "INSERT INTO accounts (name, type, balance, is_liquid, growth_rate_apr) VALUES (?1, ?2, ?3, ?4, ?5)",
      params![
        DEFAULT_ACCOUNT_NAME,
        DEFAULT_ACCOUNT_TYPE,
        DEFAULT_ACCOUNT_BALANCE,
        1,
        0.0
      ],
    )
    .context("failed seeding default account")?;
  }

  Ok(())
}

fn liquid_starting_balance(db_path: &Path) -> Result<f64> {
  let conn = Connection::open(db_path)
    .with_context(|| format!("failed opening sqlite database at {}", db_path.display()))?;

  conn.query_row(
    "SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE is_liquid = 1",
    [],
    |row| row.get(0),
  )
  .context("failed reading liquid account balance")
}

fn build_forecast_points(start_balance: f64) -> Vec<ForecastPoint> {
  let today = Utc::now().date_naive();

  (0..30)
    .map(|offset| {
      let date = today + Duration::days(offset);
      let daily_change = 16.25 * offset as f64;

      ForecastPoint {
        date: date.to_string(),
        balance: ((start_balance - daily_change) * 100.0).round() / 100.0,
      }
    })
    .collect()
}

fn forecast_from_database(db_path: &Path) -> Result<Vec<ForecastPoint>> {
  let start_balance = liquid_starting_balance(db_path)?;
  Ok(build_forecast_points(start_balance))
}

#[tauri::command]
fn forecast_30_days(app: AppHandle) -> Result<Vec<ForecastPoint>, String> {
  let db_path = database_path(&app).map_err(|err| err.to_string())?;
  bootstrap_database_at(&db_path).map_err(|err| err.to_string())?;
  forecast_from_database(&db_path).map_err(|err| err.to_string())
}

pub fn run() {
  tauri::Builder::default()
    .setup(|app| {
      let db_path = database_path(app.handle())?;
      bootstrap_database_at(&db_path)?;
      Ok(())
    })
    .invoke_handler(tauri::generate_handler![forecast_30_days])
    .run(tauri::generate_context!())
    .expect("error while running tauri app");
}

#[cfg(test)]
mod tests {
  use super::*;
  use tempfile::tempdir;

  fn read_table_names(db_path: &Path) -> Result<Vec<String>> {
    let conn = Connection::open(db_path)
      .with_context(|| format!("failed opening sqlite database at {}", db_path.display()))?;

    let mut statement = conn.prepare(
      "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
    )?;

    let names = statement
      .query_map([], |row| row.get::<_, String>(0))?
      .collect::<std::result::Result<Vec<_>, _>>()?;

    Ok(names)
  }

  #[test]
  fn bootstrap_creates_prd_schema_and_seed_account() {
    let dir = tempdir().expect("temporary directory should be created");
    let db_path = dir.path().join("test.sqlite3");

    bootstrap_database_at(&db_path).expect("database bootstrap should succeed");

    let tables = read_table_names(&db_path).expect("table names should be readable");
    assert!(tables.contains(&"accounts".to_string()));
    assert!(tables.contains(&"transactions".to_string()));
    assert!(tables.contains(&"scheduled_items".to_string()));
    assert!(tables.contains(&"budgets".to_string()));

    let conn = Connection::open(db_path).expect("database should open");
    let seeded_count: i64 = conn
      .query_row(
        "SELECT COUNT(1) FROM accounts WHERE name = ?1 AND is_liquid = 1",
        [DEFAULT_ACCOUNT_NAME],
        |row| row.get(0),
      )
      .expect("seed count should be queryable");
    assert_eq!(seeded_count, 1);
  }

  #[test]
  fn forecast_returns_30_points_from_seeded_balance() {
    let dir = tempdir().expect("temporary directory should be created");
    let db_path = dir.path().join("test.sqlite3");

    bootstrap_database_at(&db_path).expect("database bootstrap should succeed");
    let forecast = forecast_from_database(&db_path).expect("forecast should be generated");

    assert_eq!(forecast.len(), 30);
    assert_eq!(forecast[0].balance, DEFAULT_ACCOUNT_BALANCE);
    assert!(forecast[29].balance < forecast[0].balance);
  }
}
