(() => {
  const body = document.body;
  const ctx = { code: body.dataset.code || '', isDM: String(body.dataset.isdm) === 'true' };
  const list = document.getElementById('list');
  const roundEl = document.getElementById('round');
  const turnEl = document.getElementById('turn');
  const nextBtn = document.getElementById('nextBtn');

  const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/${ctx.code}`);


  // Ping Pong keepalive
  let pingInterval = setInterval(() => {
    if (ws.readyState === 1) ws.send(JSON.stringify({ type: 'ping' }));
  }, 20000); // every 20 seconds

  ws.addEventListener('message', (ev) => {
    const msg = JSON.parse(ev.data);
    if (msg.type === 'pong') return; // ignore pong
    if (msg.type === 'state') {
      const { round, turn, entries, dmUid } = msg.data;
      roundEl.textContent = round;
      turnEl.textContent = entries.length ? (turn + 1) : 0;
      render(entries, turn);
      if (!ctx.isDM) {
        document.querySelector('small.text-muted')?.classList.add('d-none');
      }
    }
  });

  function render(entries, turn) {
    list.innerHTML = '';
    entries.forEach((e, i) => {


      const li = document.createElement('li');
      li.className = `list-group-item bg-dark text-light d-flex justify-content-between align-items-center entity ${e.type}`;
      li.dataset.id = e.id;

      // monster entities have colored border based on HP percentage
      if (e.type === 'monster') {
        li.style.border = '2px solid rgba(0, 170, 170, 1)';
        if (e.hp < e.maxHp / 4) {
          li.style.borderColor = 'rgba(221, 34, 0, 0.8)';
        } else if (e.hp < e.maxHp / 2) {
          li.style.borderColor = 'rgba(170, 136, 0, 1)';
        }
      }

      const left = document.createElement('div');
      left.className = 'd-flex flex-column';

      // Entity name and initiative
      const nameDiv = document.createElement('div');
      nameDiv.className = 'fw-semibold';
      nameDiv.textContent = e.name;
      left.appendChild(nameDiv);

      // Initiative info
      const initDiv = document.createElement('div');
      initDiv.className = 'small text-secondary';
      initDiv.textContent = `Init ${e.initiative}${e.bonus ? ` (b+${e.bonus})` : ''}`;
      left.appendChild(initDiv);

      // Tags (conditions)
      if (e.tags && e.tags.length > 0) {
        const tagsDiv = document.createElement('div');
        tagsDiv.className = 'small mt-1';
        e.tags.forEach(tag => {
          const tagSpan = document.createElement('span');
          tagSpan.className = 'badge text-bg-warning me-1';
          tagSpan.textContent = tag;
          if (ctx.isDM) {
            tagSpan.style.cursor = 'pointer';
            tagSpan.title = 'Click to remove';
            tagSpan.onclick = () => {
              if (confirm(`Remove condition "${tag}"?`)) {
                wsSend('removeEntityTag', { id: e.id, tag });
              }
            };
          }
          tagsDiv.appendChild(tagSpan);
        });
        left.appendChild(tagsDiv);
      }

      li.appendChild(left);

      const right = document.createElement('div');
      right.className = 'd-flex align-items-center gap-2';

      // HP display for monsters
      // Damage button for monsters (DM only)
      if (e.type === 'monster' && ctx.isDM) {


        const hpWrap = document.createElement('div');
        hpWrap.style.minWidth = '120px';
        hpWrap.className = 'text-end';
        hpWrap.innerHTML = `<div class="small">HP ${e.hp}/${e.maxHp}</div>
                            <div class="hpbar"><div class="inner" style="width:${pct(e.hp, e.maxHp)}%"></div></div>`;
        right.appendChild(hpWrap);

        const dmgBtn = document.createElement('button');
        dmgBtn.className = 'btn btn-sm btn-outline-danger';
        dmgBtn.textContent = 'Damage';
        dmgBtn.onclick = () => {
          const v = parseInt(prompt('Damage amount?') || '0', 10);
          if (!Number.isFinite(v) || v <= 0) return;
          wsSend('damage', { id: e.id, dmg: v });
        };
        right.appendChild(dmgBtn);
      }

      // GM Controls
      if (ctx.isDM) {
        const controlsDiv = document.createElement('div');
        controlsDiv.className = 'dropdown';

        const dropBtn = document.createElement('button');
        dropBtn.className = 'btn btn-sm btn-outline-secondary dropdown-toggle';
        dropBtn.setAttribute('data-bs-toggle', 'dropdown');
        dropBtn.textContent = '⚙️';

        const dropMenu = document.createElement('ul');
        dropMenu.className = 'dropdown-menu dropdown-menu-dark';

        // Rename option
        const renameItem = document.createElement('li');
        const renameLink = document.createElement('a');
        renameLink.className = 'dropdown-item';
        renameLink.href = '#';
        renameLink.textContent = 'Rename';
        renameLink.onclick = (ev) => {
          ev.preventDefault();
          const newName = prompt('New name:', e.name);
          if (newName && newName.trim() !== e.name) {
            wsSend('renameEntity', { id: e.id, name: newName.trim() });
          }
        };
        renameItem.appendChild(renameLink);
        dropMenu.appendChild(renameItem);

        // Edit HP option (for monsters)
        if (e.type === 'monster') {
          const hpItem = document.createElement('li');
          const hpLink = document.createElement('a');
          hpLink.className = 'dropdown-item';
          hpLink.href = '#';
          hpLink.textContent = 'Edit HP';
          hpLink.onclick = (ev) => {
            ev.preventDefault();
            const currentHP = prompt('Current HP:', e.hp);
            const maxHP = prompt('Max HP:', e.maxHp);
            if (currentHP !== null && maxHP !== null) {
              const hp = parseInt(currentHP, 10) || 0;
              const maxHp = parseInt(maxHP, 10) || 0;
              wsSend('editEntityHP', { id: e.id, hp, maxHp });
            }
          };
          hpItem.appendChild(hpLink);
          dropMenu.appendChild(hpItem);
        }

        // Add condition option
        const tagItem = document.createElement('li');
        const tagLink = document.createElement('a');
        tagLink.className = 'dropdown-item';
        tagLink.href = '#';
        tagLink.textContent = 'Add Condition';
        tagLink.onclick = (ev) => {
          ev.preventDefault();
          const tag = prompt('Condition name (e.g., poisoned, stunned):');
          if (tag && tag.trim()) {
            wsSend('addEntityTag', { id: e.id, tag: tag.trim() });
          }
        };
        tagItem.appendChild(tagLink);
        dropMenu.appendChild(tagItem);

        // Delete option
        const deleteItem = document.createElement('li');
        const deleteLink = document.createElement('a');
        deleteLink.className = 'dropdown-item text-danger';
        deleteLink.href = '#';
        deleteLink.textContent = 'Delete';
        deleteLink.onclick = (ev) => {
          ev.preventDefault();
          if (confirm(`Delete "${e.name}"?`)) {
            wsSend('deleteEntity', { id: e.id });
          }
        };
        deleteItem.appendChild(deleteLink);
        dropMenu.appendChild(deleteItem);

        controlsDiv.appendChild(dropBtn);
        controlsDiv.appendChild(dropMenu);
        right.appendChild(controlsDiv);
      }

      li.appendChild(right);
      if (i === turn) li.classList.add('active');
      list.appendChild(li);
    });
  }

  function wsSend(type, data) {
    ws.readyState === 1 && ws.send(JSON.stringify({ type, data }));
  }

  function escapeHtml(s) { return s.replace(/[&<>"]+/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c])); }
  function pct(a, b) { if (!b) return 0; return Math.max(0, Math.min(100, Math.round(a * 100 / b))); }

  // Forms
  const pf = document.getElementById('playerForm');
  pf?.addEventListener('submit', (e) => {
    e.preventDefault();
    const fd = new FormData(pf);
    const name = (fd.get('name') || '').toString();
    const initiative = Math.max(0, parseInt(fd.get('initiative') || '0', 10));
    const bonus = parseInt(fd.get('bonus') || '0', 10) || 0;
    wsSend('addPlayer', { name, initiative, bonus });
    pf.reset();
  });
  document.getElementById('rollBtn')?.addEventListener('click', () => {
    const fd = new FormData(pf);
    const name = (fd.get('name') || '').toString();
    const bonus = parseInt(fd.get('bonus') || '0', 10) || 0;
    wsSend('addPlayerRoll', { name, bonus });
    pf.reset();
  });

  const mf = document.getElementById('monsterForm');
  mf?.addEventListener('submit', (e) => {
    e.preventDefault();
    const fd = new FormData(mf);
    const name = (fd.get('name') || '').toString();
    const hp = parseInt(fd.get('hp') || '0', 10) || 0;
    const initiative = Math.max(0, parseInt(fd.get('initiative') || '0', 10));
    const bonus = parseInt(fd.get('bonus') || '0', 10) || 0;
    wsSend('addMonster', { name, hp, initiative, bonus });
    mf.reset();
  });

  nextBtn?.addEventListener('click', () => wsSend('next', {}));

  // Reset button (DM only)
  const resetBtn = document.getElementById('resetBtn');
  resetBtn?.addEventListener('click', () => {
    if (confirm('Are you sure you want to reset the initiative? This will remove all players and monsters.')) {
      wsSend('reset', {});
    }
  });

  // Drag and drop for DM
  if (ctx.isDM) {
    new Sortable(list, {
      animation: 150,
      onEnd: () => {
        const order = Array.from(list.children).map(li => li.dataset.id);
        wsSend('reorder', { order });
      }
    });
  }
})();

// Reconnecting WebSocket with visibility and online checks
class reconnectWebsocket {
  constructor(url) {
    this.url = url;
    this.WebSocket = null;
    this.retryDelay = 1000; // initial delay
    this.maxDelay = 30000; // max delay

    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'visible' && !this.WebSocket) {
        this.checkConnection();
      }
    });

    window.addEventListener('online', () => this.checkConnection());
      this.connect();
    }
  
    connect() {
      this.WebSocket = new WebSocket(this.url);

      this.WebSocket.onopen = () => {
        console.log('WebSocket connected');
        this.retryDelay = 1000; // reset delay on successful connection
      };

      this.WebSocket.onclose = () => {
        console.log('WebSocket disconnected, attempting to reconnect...');
        this.WebSocket = null;
        setTimeout(() => this.connect(), this.retryDelay);
        this.retryDelay = Math.min(this.retryDelay * 2, this.maxDelay); // exponential backoff
      };

      this.WebSocket.onerror = (err) => {
        console.error('WebSocket error:', err);
        this.WebSocket.close();
      };
    }

    checkConnection() {
      if (!this.WebSocket || this.WebSocket.readyState === WebSocket.CLOSED) { this.Connect(); }
    }

    scheduleReconnect() {

      const jitter = Math.random() * 1000; // up to 1 second of random jitter
      const delay = Math.min(this.retryDelay + jitter, this.maxDelay);

      console.log('Reconnecting... Next attempt in', Math.round(delay), 'ms');

      setTimeout(() => {
        this.retryDelay *= 2; // exponential backoff
        this.checkConnection();
      }, delay);
    }
  }