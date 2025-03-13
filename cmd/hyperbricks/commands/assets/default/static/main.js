let lastIndex = null;

function getNextRandomIndex(range) {
    let i;
    do {
        i = Math.floor(Math.random() * range);
    } while (i === lastIndex); // Ensure it's different from the last one

    lastIndex = i; // Store last used index
    return i;
}

function refreshBackground() {
    const container = document.querySelector('.fade-in-background');
    const newUrl = "https://picsum.photos/1920/1080?random=" + new Date().getTime();
    // Remove the current animation
    container.style.animation = 'none';

    // Force a reflow to reset the animation state
    void container.offsetHeight; // or container.getBoundingClientRect();

    // Reapply the animation
    container.style.animation = "fadeInBackground 2s ease-in-out forwards";

    // Update the background image with a cache-busting parameter if needed
    container.style.backgroundImage = "url('" + newUrl + "?t=" + Date.now() + "')";
}
function myCustomEvent(range = 5) {
    const i = getNextRandomIndex(range);
    htmx.ajax('GET', `/get-quote?id=${i}`, {
        target: "#quote-box",  // Only updates #quote-box
        swap: "innerHTML"      // Prevents removing the element
    });

    refreshBackground()
}

