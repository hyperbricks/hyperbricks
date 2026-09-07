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

function updateStatusDemoRequest(event) {
  const path = event.detail?.ctx?.request?.action
  if (path) {
    setText('pattern-last-fragment', path)
  }
}

function installStatusDemoDiagnostics() {
  document.addEventListener('DOMContentLoaded', updateStatusDemoLocation)
  document.addEventListener('htmx:after:history:update', updateStatusDemoLocation)
  document.addEventListener('htmx:after:swap', (event) => {
    updateStatusDemoLocation()
    updateStatusDemoRequest(event)
  })
  document.addEventListener('htmx:before:request', updateStatusDemoRequest)
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

  document.addEventListener('htmx:after:settle', (event) => {
    const target = event.detail?.task?.target
    const rightColumn = document.querySelector('#right_column')
    if (!(target instanceof Element && rightColumn instanceof Element &&
      (rightColumn.contains(target) || target.contains(rightColumn)))) {
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
