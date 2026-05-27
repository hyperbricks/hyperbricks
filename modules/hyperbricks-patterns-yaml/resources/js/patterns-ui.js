function setText(id, value) {
  const node = document.getElementById(id)
  if (node) {
    node.textContent = value
  }
}

function updateStatusDemoLocation() {
  if (!document.getElementById('pattern-current-url')) {
    return
  }

  setText('pattern-current-url', window.location.pathname + window.location.search)
}

function installStatusDemoDiagnostics() {
  document.addEventListener('DOMContentLoaded', updateStatusDemoLocation)
  document.body.addEventListener('htmx:pushedIntoHistory', updateStatusDemoLocation)
  document.body.addEventListener('htmx:afterSwap', (event) => {
    if (event.target && event.target.id === 'status-demo-panel') {
      updateStatusDemoLocation()
    }
  })
  document.body.addEventListener('htmx:beforeRequest', (event) => {
    const path = event.detail && event.detail.pathInfo ? event.detail.pathInfo.requestPath : ''
    if (path) {
      setText('pattern-last-fragment', path)
    }
  })
}

function currentSectionHash() {
  const hash = window.location.hash || ''
  return hash.startsWith('#') && hash.length > 1 ? hash.slice(1) : ''
}

function escapedSelectorID(id) {
  if (typeof window.CSS !== 'undefined' && typeof window.CSS.escape === 'function') {
    return window.CSS.escape(id)
  }

  return String(id).replace(/([ !"#$%&'()*+,./:;<=>?@[\\\]^`{|}~])/g, '\\$1')
}

let pendingRailHash = ''

function scrollRightColumnToSection(hash, behavior) {
  const targetID = String(hash || '').trim()
  if (!targetID) {
    return
  }

  const rightColumn = document.querySelector('#right_column')
  if (!(rightColumn instanceof HTMLElement)) {
    return
  }

  const section = rightColumn.querySelector(`#${escapedSelectorID(targetID)}`)
  if (!(section instanceof HTMLElement)) {
    return
  }

  const top =
    section.getBoundingClientRect().top -
    rightColumn.getBoundingClientRect().top +
    rightColumn.scrollTop -
    16

  rightColumn.scrollTo({
    top: Math.max(0, top),
    behavior: behavior || 'smooth',
  })
}

function applySectionHashScroll(behavior) {
  if (!document.querySelector('[data-section-rail]')) {
    return
  }

  const hash = pendingRailHash || currentSectionHash()
  if (!hash) {
    return
  }

  window.requestAnimationFrame(() => {
    scrollRightColumnToSection(hash, behavior || 'smooth')
    pendingRailHash = ''
  })
}

function installSectionRailScrolling() {
  document.addEventListener('click', (event) => {
    const link = event.target && event.target.closest ? event.target.closest('[data-outline-link]') : null
    if (!(link instanceof HTMLAnchorElement)) {
      return
    }

    const href = link.getAttribute('href') || ''
    if (!href.includes('#')) {
      return
    }

    pendingRailHash = href.slice(href.indexOf('#') + 1).trim()
  })

  document.body.addEventListener('htmx:afterSettle', (event) => {
    const target = event.target instanceof Element ? event.target : null
    const rightColumn = document.querySelector('#right_column')
    if (!(target && rightColumn instanceof Element && rightColumn.contains(target))) {
      return
    }

    applySectionHashScroll(target.id === 'right_column' ? 'smooth' : 'auto')
  })

  window.addEventListener('hashchange', () => {
    applySectionHashScroll('smooth')
  })

  window.requestAnimationFrame(() => {
    applySectionHashScroll('auto')
  })
}

installStatusDemoDiagnostics()
installSectionRailScrolling()
