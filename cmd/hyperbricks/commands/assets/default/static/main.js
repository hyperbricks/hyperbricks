let lastIndex = null;
let activeBackground = 1;
let isTransitioning = false; // Prevents triggering during fade

function getNextRandomIndex(range) {
    let i;
    do {
        i = Math.floor(Math.random() * range);
    } while (i === lastIndex);

    lastIndex = i;
    return i;
}

function refreshBackground(callback) {
    if (isTransitioning) return; // Prevents re-triggering

    isTransitioning = true; // Block new triggers

    const bg1 = document.getElementById('background1');
    const bg2 = document.getElementById('background2');

    if (!bg1 || !bg2) {
        console.error("Background elements not found!");
        isTransitioning = false;
        return;
    }

    const current = activeBackground === 1 ? bg1 : bg2;
    const next = activeBackground === 1 ? bg2 : bg1;

    const newUrl = `https://picsum.photos/1920/1080?random=${new Date().getTime()}`;
    const img = new Image();
    img.src = newUrl;

    img.onload = () => {
        next.style.backgroundImage = `url('${newUrl}')`;

        next.style.opacity = "1";
        current.style.opacity = "0";

        // Wait for fade-out to complete (2s) before allowing new triggers
        setTimeout(() => {
            current.style.backgroundImage = "";
            isTransitioning = false; // Allow new triggers
            if (callback) callback(); // Execute callback after transition
        }, 2000);
    };

    activeBackground = activeBackground === 1 ? 2 : 1;
}

function myCustomEvent() {
    if (isTransitioning) return;
    const i = getNextRandomIndex(range = 5);
        htmx.ajax('GET', `/get-quote?id=${i}`, {
            target: "#quote-box",
            swap: "innerHTML"
        });
    refreshBackground(() => {
        
    });
}