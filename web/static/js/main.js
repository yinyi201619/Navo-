// Navo NT Forum - 前端交互 JS

(function () {
  'use strict';

  // 签到
  window.doCheckin = function () {
    fetch('/api/checkin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin'
    })
      .then(function (r) { return r.json(); })
      .then(function (res) {
        var el = document.getElementById('checkin-result');
        if (res.code === 0) {
          el.style.color = '#10b981';
          el.textContent = '签到成功！连续 ' + res.data.continuous + ' 天，获得 ' + res.data.points + ' 积分';
        } else {
          el.style.color = '#ef4444';
          el.textContent = res.message || '签到失败';
        }
      })
      .catch(function () {
        document.getElementById('checkin-result').textContent = '请先登录';
      });
  };

  // 点赞帖子
  window.likeTopic = function (id) {
    fetch('/api/topics/' + id + '/like', {
      method: 'POST',
      credentials: 'same-origin'
    })
      .then(function (r) { return r.json(); })
      .then(function (res) {
        if (res.code === 0) {
          var btn = document.querySelector('[data-topic-like="' + id + '"]');
          if (btn) {
            btn.classList.toggle('active', res.data.liked);
            var cnt = btn.querySelector('.like-count');
            if (cnt) cnt.textContent = res.data.count;
          }
        }
      });
  };

  // 收藏帖子
  window.favTopic = function (id) {
    fetch('/api/topics/' + id + '/favorite', {
      method: 'POST',
      credentials: 'same-origin'
    })
      .then(function (r) { return r.json(); })
      .then(function (res) {
        if (res.code === 0) {
          var btn = document.querySelector('[data-topic-fav="' + id + '"]');
          if (btn) btn.classList.toggle('active', res.data.favorited);
        }
      });
  };

  // 点赞回复
  window.likeReply = function (id) {
    fetch('/api/replies/' + id + '/like', {
      method: 'POST',
      credentials: 'same-origin'
    })
      .then(function (r) { return r.json(); })
      .then(function (res) {
        if (res.code === 0) {
          var btn = document.querySelector('[data-reply-like="' + id + '"]');
          if (btn) {
            btn.classList.toggle('active', res.data.liked);
            var cnt = btn.querySelector('.like-count');
            if (cnt) cnt.textContent = res.data.count;
          }
        }
      });
  };

  // 回复框聚焦到指定回复
  window.replyTo = function (floor, author, parentId) {
    var ta = document.getElementById('reply-content');
    if (ta) {
      ta.value = '@' + author + ' ';
      ta.focus();
    }
    var pi = document.getElementById('reply-parent');
    if (pi) pi.value = parentId;
  };

  // 平滑滚动
  document.querySelectorAll('a[href^="#"]').forEach(function (a) {
    a.addEventListener('click', function (e) {
      var target = document.querySelector(this.getAttribute('href'));
      if (target) {
        e.preventDefault();
        target.scrollIntoView({ behavior: 'smooth' });
      }
    });
  });

  // 淡入动画触发
  if ('IntersectionObserver' in window) {
    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
      if (entry.isIntersecting) {
        entry.target.style.opacity = '1';
        entry.target.style.transform = 'translateY(0)';
      }
    });
  }, { threshold: 0.1 });
    document.querySelectorAll('.fade-in').forEach(function (el) {
      el.style.opacity = '0';
      el.style.transform = 'translateY(20px)';
      el.style.transition = 'opacity .5s ease, transform .5s ease';
      observer.observe(el);
    });
  }
})();
