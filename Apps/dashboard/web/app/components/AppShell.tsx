// @ts-nocheck
"use client";

import { useState, useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import {
  Activity,
  Play,
  LayoutDashboard,
  Menu,
  Sun,
  Moon,
  Monitor
} from "lucide-react";

const API_URL = "http://localhost:8080/api";

export default function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();

  const [theme, setTheme] = useState("dark");
  const [mainSidebarOpen, setMainSidebarOpen] = useState(false);
  const [simStatus, setSimStatus] = useState("idle");

  useEffect(() => {
    document.documentElement.classList.remove('light', 'dark');
    document.documentElement.classList.add(theme);
  }, [theme]);

  const handleStartSim = () => {
    setSimStatus("running");
    fetch(`${API_URL}/simulation/start`, { method: 'POST' })
      .then(() => {
        setTimeout(() => setSimStatus("completed"), 2000);
      });
  };

  return (
    <div
      className={`dashboard-layout ${theme}`}
      style={{
        background: theme === 'dark' ? '#0a0a0c' : '#f8fafc',
        color: theme === 'dark' ? '#ededed' : '#0f172a'
      }}
    >
      {/* Top Header */}
      <header className="top-header">
        <button
          className="btn-ghost"
          style={{
            marginRight: '1rem',
            border: '0 none',
            outline: 'none',
            boxShadow: 'none',
            background: 'transparent',
            padding: '8px',
            borderRadius: '8px'
          }}
          onClick={() => setMainSidebarOpen(!mainSidebarOpen)}
        >
          <Menu size={24} />
        </button>

        <Activity color="#6366f1" size={24} />

        <div className="header-title">SATIP</div>

        <div style={{ marginLeft: 'auto', display: 'flex', gap: '12px' }}>
          <button className="btn" onClick={handleStartSim} disabled={simStatus === "running"}>
            <Play size={16} fill="currentColor" />
            {simStatus === "running" ? "Simulando..." : "Iniciar"}
          </button>
        </div>
      </header>

      {/* Main Sidebar (Navigation & Theme) */}
      <aside className="main-sidebar" style={{ width: mainSidebarOpen ? 'var(--sidebar-w)' : 'var(--sidebar-collapsed)' }}>
        <nav style={{ padding: '0.75rem', display: 'flex', flexDirection: 'column', gap: '8px', flex: 1 }}>
          <div
            className={`eq-list-item ${pathname === '/' ? 'active' : ''}`}
            style={{ borderRadius: '12px', padding: '12px', cursor: 'pointer' }}
            onClick={() => router.push('/')}
          >
            <LayoutDashboard size={20} />
            <span style={{ display: mainSidebarOpen ? 'block' : 'none', marginLeft: '12px' }}>Dashboard</span>
          </div>
          <div
            className={`eq-list-item ${pathname === '/processos' ? 'active' : ''}`}
            style={{ borderRadius: '12px', padding: '12px', cursor: 'pointer' }}
            onClick={() => router.push('/processos')}
          >
            <Monitor size={20} />
            <span style={{ display: mainSidebarOpen ? 'block' : 'none', marginLeft: '12px' }}>Processos</span>
          </div>
        </nav>

        <div style={{ padding: '1rem', borderTop: '1px solid var(--border)' }}>
          <button
            className="btn-ghost"
            style={{
              width: '100%',
              justifyContent: mainSidebarOpen ? 'flex-start' : 'center',
              padding: '12px',
              borderRadius: '12px',
              border: '0 none',
              outline: 'none',
              boxShadow: 'none',
              background: 'transparent',
              color: '#94a3b8'
            }}
            onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          >
            {theme === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
            {mainSidebarOpen && <span style={{ marginLeft: '12px' }}>Modo {theme === 'dark' ? 'Claro' : 'Escuro'}</span>}
          </button>
        </div>
      </aside>

      {children}

      <style jsx>{`
        .dashboard-layout {
          transition: all 0.3s ease;
        }
      `}</style>
    </div>
  );
}
