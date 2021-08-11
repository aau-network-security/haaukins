const inputs = document.querySelectorAll(".input");


function addcl(){
    let parent = this.parentNode.parentNode;
    parent.classList.add("focus");
}

function remcl(){
    let parent = this.parentNode.parentNode;
    if(this.value == ""){
        parent.classList.remove("focus");
    }
}


inputs.forEach(input => {
    input.addEventListener("focus", addcl);
    input.addEventListener("blur", remcl);
});


let navbarCollapse = function () {
    let navbar = document.getElementById("mainNav");
    let content = document.getElementById("content");

    if (content.getBoundingClientRect().top < 0){
        navbar.classList.add("navbar-shrink")
    }else{
        navbar.classList.remove("navbar-shrink")
    }
}

navbarCollapse();
window.onscroll = function() {
    navbarCollapse();
};