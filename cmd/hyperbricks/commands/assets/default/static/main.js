let lastIndex = null;
let activeBackground = 1;
let isTransitioning = false;
let backgroundTimeout = null; // Prevent timeout stacking

function getNextRandomIndex(range) {
    let i;
    do {
        i = Math.floor(Math.random() * range);
    } while (i === lastIndex);

    lastIndex = i;
    return i;
}

function refreshBackground(callback) {
    if (isTransitioning) return;

    isTransitioning = true;
    const bg1 = document.getElementById('background1');
    const bg2 = document.getElementById('background2');
    const button = document.getElementById('quote-button');

    if (!bg1 || !bg2 || !button) {
        console.error("Background elements or button not found!");
        isTransitioning = false;
        return;
    }

    // Change button color immediately on click
    button.classList.replace("bg-black", "bg-white");
    button.classList.replace("text-white", "text-black");

    const current = activeBackground === 1 ? bg1 : bg2;
    const next = activeBackground === 1 ? bg2 : bg1;

    const newUrl = `https://picsum.photos/1920/1080?random=${new Date().getTime()}`;
    const img = new Image();
    img.src = newUrl;

    img.onload = () => {
        next.style.backgroundImage = `url('${newUrl}')`;
        next.style.opacity = "1";
        current.style.opacity = "0";

        // Clear any existing timeout before setting a new one
        if (backgroundTimeout) clearTimeout(backgroundTimeout);

        backgroundTimeout = setTimeout(() => {
            current.style.backgroundImage = "";
            isTransitioning = false;

            // Smooth fade back to black
            button.classList.replace("bg-white", "bg-black");
            button.classList.replace("text-black", "text-white");

            if (callback) callback();
        }, 2000);

        // Cleanup image object to prevent memory leak
        img.onload = null;
    };

    activeBackground = activeBackground === 1 ? 2 : 1;
}

function myCustomEvent(range = 5) {
    if (isTransitioning) return;

    const quoteBox = document.getElementById("quote-box");
    quoteBox.classList.remove("opacity-100", "transition-opacity", "duration-500");
    quoteBox.classList.add("opacity-0", "transition-opacity", "duration-500");
    backgroundTimeout = setTimeout(() => {
        const i = getNextRandomIndex(range);
        htmx.ajax('GET', `/get-quote?id=${i}`).then(() => {
            refreshBackground(() => {
                quoteBox.classList.remove("opacity-0", "transition-opacity", "duration-500");
                quoteBox.classList.add("opacity-100", "transition-opacity", "duration-500");
            });
        })
    }, 500);
}