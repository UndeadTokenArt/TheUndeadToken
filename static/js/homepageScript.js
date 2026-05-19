document.addEventListener("DOMContentLoaded", function() {
    const body = document.body;
    const overlay = document.getElementById("sidebar-overlay");
    const toggle = document.getElementById("sidebar-toggle");

    function closeSidebar() {
        body.classList.add("sidebar-collapsed");
    }

    function openSidebar() {
        body.classList.remove("sidebar-collapsed");
    }

    toggle.addEventListener("click", function() {
        body.classList.toggle("sidebar-collapsed");
    });

    overlay.addEventListener("click", closeSidebar);

    document.querySelectorAll('a[href^="#"]').forEach(function(anchor) {
        anchor.addEventListener("click", function(e) {
            const targetId = this.getAttribute("href");
            if (targetId === "#") {
                return;
            }

            const targetElement = document.querySelector(targetId);
            if (!targetElement) {
                return;
            }

            e.preventDefault();
            targetElement.scrollIntoView({ behavior: "smooth", block: "start" });

            if (window.innerWidth <= 960) {
                closeSidebar();
            }
        });
    });

    window.addEventListener("resize", function() {
        if (window.innerWidth > 960) {
            openSidebar();
        }
    });

    if (window.innerWidth <= 960) {
        closeSidebar();
    } else {
        openSidebar();
    }
});