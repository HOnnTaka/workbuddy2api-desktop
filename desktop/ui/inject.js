(function() {
  if (window.__WB2API_DESKTOP_INJECTED__) return;
  window.__WB2API_DESKTOP_INJECTED__ = true;

  // 1. 全局注入极细现代化平滑滚动条（双重保障）
  const styleEl = document.createElement('style');
  styleEl.id = '__wb2api_desktop_theme_clean__';
  styleEl.textContent = 
    html, body, *, *::before, *::after {
      scrollbar-width: thin !important;
      scrollbar-color: rgba(148, 163, 184, 0.3) transparent !important;
    }
    ::-webkit-scrollbar,
    *::-webkit-scrollbar {
      width: 5px !important;
      height: 5px !important;
    }
    ::-webkit-scrollbar-track,
    *::-webkit-scrollbar-track {
      background: transparent !important;
    }
    ::-webkit-scrollbar-thumb,
    *::-webkit-scrollbar-thumb {
      background: rgba(148, 163, 184, 0.3) !important;
      border-radius: 9999px !important;
      border: 1px solid transparent !important;
      background-clip: padding-box !important;
      transition: background-color 0.2s ease !important;
    }
    ::-webkit-scrollbar-thumb:hover,
    *::-webkit-scrollbar-thumb:hover {
      background: rgba(91, 124, 250, 0.75) !important;
    }
    ::-webkit-scrollbar-corner,
    *::-webkit-scrollbar-corner {
      background: transparent !important;
    }

    [data-theme="light"] *,
    [data-theme="light"] html,
    [data-theme="light"] body {
      scrollbar-color: rgba(100, 116, 139, 0.35) transparent !important;
    }
    [data-theme="light"] ::-webkit-scrollbar-thumb,
    [data-theme="light"] *::-webkit-scrollbar-thumb {
      background: rgba(100, 116, 139, 0.35) !important;
    }
    [data-theme="light"] ::-webkit-scrollbar-thumb:hover,
    [data-theme="light"] *::-webkit-scrollbar-thumb:hover {
      background: rgba(59, 91, 219, 0.75) !important;
    }
  ;
  document.head.appendChild(styleEl);

  // 2. 托盘联动接口
  window.__wb2api_show_about = function() {
    if (typeof window.go === 'function') {
      window.go('about');
      history.replaceState(null, '', '#about');
    } else {
      location.hash = '#about';
    }
  };

  window.__wb2api_trigger_restart_ui = function() {
    const shield = document.getElementById('reconnectShield');
    if (shield) {
      shield.style.display = 'flex';
      const countEl = document.getElementById('reconnectCount');
      if (countEl) countEl.textContent = '托盘已下发重启命令，正在重新连接...';
    }
  };
})();
