(function(e){function t(t){for(var a,o,l=t[0],i=t[1],c=t[2],d=0,p=[];d<l.length;d++)o=l[d],Object.prototype.hasOwnProperty.call(s,o)&&s[o]&&p.push(s[o][0]),s[o]=0;for(a in i)Object.prototype.hasOwnProperty.call(i,a)&&(e[a]=i[a]);u&&u(t);while(p.length)p.shift()();return r.push.apply(r,c||[]),n()}function n(){for(var e,t=0;t<r.length;t++){for(var n=r[t],a=!0,l=1;l<n.length;l++){var i=n[l];0!==s[i]&&(a=!1)}a&&(r.splice(t--,1),e=o(o.s=n[0]))}return e}var a={},s={0:0},r=[];function o(t){if(a[t])return a[t].exports;var n=a[t]={i:t,l:!1,exports:{}};return e[t].call(n.exports,n,n.exports,o),n.l=!0,n.exports}o.m=e,o.c=a,o.d=function(e,t,n){o.o(e,t)||Object.defineProperty(e,t,{enumerable:!0,get:n})},o.r=function(e){"undefined"!==typeof Symbol&&Symbol.toStringTag&&Object.defineProperty(e,Symbol.toStringTag,{value:"Module"}),Object.defineProperty(e,"__esModule",{value:!0})},o.t=function(e,t){if(1&t&&(e=o(e)),8&t)return e;if(4&t&&"object"===typeof e&&e&&e.__esModule)return e;var n=Object.create(null);if(o.r(n),Object.defineProperty(n,"default",{enumerable:!0,value:e}),2&t&&"string"!=typeof e)for(var a in e)o.d(n,a,function(t){return e[t]}.bind(null,a));return n},o.n=function(e){var t=e&&e.__esModule?function(){return e["default"]}:function(){return e};return o.d(t,"a",t),t},o.o=function(e,t){return Object.prototype.hasOwnProperty.call(e,t)},o.p="/";var l=window["webpackJsonp"]=window["webpackJsonp"]||[],i=l.push.bind(l);l.push=t,l=l.slice();for(var c=0;c<l.length;c++)t(l[c]);var u=i;r.push([131,1]),n()})({105:function(e,t,n){var a=n(164);a.__esModule&&(a=a.default),"string"===typeof a&&(a=[[e.i,a,""]]),a.locals&&(e.exports=a.locals);var s=n(15).default;s("6aa89a66",a,!1,{sourceMap:!1,shadowMode:!1})},112:function(e,t,n){var a=n(183);a.__esModule&&(a=a.default),"string"===typeof a&&(a=[[e.i,a,""]]),a.locals&&(e.exports=a.locals);var s=n(15).default;s("08eb1e75",a,!1,{sourceMap:!1,shadowMode:!1})},113:function(e,t,n){var a=n(185);a.__esModule&&(a=a.default),"string"===typeof a&&(a=[[e.i,a,""]]),a.locals&&(e.exports=a.locals);var s=n(15).default;s("edc1ddda",a,!1,{sourceMap:!1,shadowMode:!1})},114:function(e,t,n){var a=n(187);a.__esModule&&(a=a.default),"string"===typeof a&&(a=[[e.i,a,""]]),a.locals&&(e.exports=a.locals);var s=n(15).default;s("76498982",a,!1,{sourceMap:!1,shadowMode:!1})},115:function(e,t,n){var a=n(189);a.__esModule&&(a=a.default),"string"===typeof a&&(a=[[e.i,a,""]]),a.locals&&(e.exports=a.locals);var s=n(15).default;s("2434c3f1",a,!1,{sourceMap:!1,shadowMode:!1})},117:function(e,t,n){var a=n(195);a.__esModule&&(a=a.default),"string"===typeof a&&(a=[[e.i,a,""]]),a.locals&&(e.exports=a.locals);var s=n(15).default;s("75c852a7",a,!1,{sourceMap:!1,shadowMode:!1})},126:function(e,t,n){var a=n(216);a.__esModule&&(a=a.default),"string"===typeof a&&(a=[[e.i,a,""]]),a.locals&&(e.exports=a.locals);var s=n(15).default;s("e542a700",a,!1,{sourceMap:!1,shadowMode:!1})},131:function(e,t,n){e.exports=n(217)},163:function(e,t,n){"use strict";n(105)},164:function(e,t,n){var a=n(13);t=a(!1),t.push([e.i,"\n.mybtn:focus{\n    border-color: rgba(33, 26, 82, 0.8)!important;\n    -webkit-box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(33, 26, 82, 0.6) !important;\n            box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(33, 26, 82, 0.6) !important;\n    outline: 0 none!important;\n}\n.flagSuccess{\n    border-color: rgba(33, 26, 82, 0.8)!important;\n    -webkit-box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(33, 26, 82, 0.6) !important;\n            box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(33, 26, 82, 0.6) !important;\n    outline: 0 none!important;\n}\n.flagError{\n    border-color: rgba(220, 53, 69, 0.8)!important;\n    -webkit-box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(220, 53, 69, 0.6) !important;\n            box-shadow: 0 1px 1px rgba(0, 0, 0, 0.075) inset, 0 0 8px rgba(220, 53, 69, 0.6) !important;\n    outline: 0 none!important;\n}\n.nofocus:focus{\n    -webkit-box-shadow: none!important;\n            box-shadow: none!important;\n    outline: none!important;\n}\n.icon-flag{\n    position: absolute;\n    margin-left: 10px;\n    height: 38px;\n    display: -webkit-box;\n    display: -ms-flexbox;\n    display: flex;\n    -webkit-box-align: center;\n        -ms-flex-align: center;\n            align-items: center;\n    z-index: 1000;\n}\n\n",""]),e.exports=t},182:function(e,t,n){"use strict";n(112)},183:function(e,t,n){var a=n(13);t=a(!1),t.push([e.i,"\n.even {\n    background-color: #ffffff;\n}\n.odd {\n    background-color: rgb(233, 235, 245);\n}\n.bg-isUser {\n    background-color: rgb(195, 203, 228)!important;\n}\n",""]),e.exports=t},184:function(e,t,n){"use strict";n(113)},185:function(e,t,n){var a=n(13);t=a(!1),t.push([e.i,"\ntable#scoreboardtable {\n    table-layout: fixed!important;\n    min-width: 1800px!important;\n}\n.table .thead-dark-custom th{\n    color:#fff!important;\n    background-color:#211A52;\n    border-bottom: none;\n    color:inherit;\n}\n.chal-points-font{\n    font-family: 'Orbitron', sans-serif !important;\n    letter-spacing: 1px;\n}\n",""]),e.exports=t},186:function(e,t,n){"use strict";n(114)},187:function(e,t,n){var a=n(13);t=a(!1),t.push([e.i,"\na[data-v-eec7658c] {\n    color: #211A52;\n    text-decoration: none;\n}\n",""]),e.exports=t},188:function(e,t,n){"use strict";n(115)},189:function(e,t,n){var a=n(13);t=a(!1),t.push([e.i,"\n.btn-haaukins{\n  color: #fff;\n  background-color: #211A52;\n  border-color: #211A52;\n}\n.btn-haaukins:hover{\n  color: #fff;\n  background-color: #1a1441;\n  border-color: #1a1441;\n}\n.btn-disabled{\n  color: #fff;\n  background-color: #949494;\n  border-color: #565667;\n}\n.btn-disabled:hover{\n  color: #fff;\n  background-color: #6c6499;\n  border-color: #1a1441;\n}\n.btn-success{\n  background-color: #6ab55f;\n  border-color: #6ab55f;\n}\n.btn-success:hover{\n  background-color: #55a04a;\n  border-color: #55a04a;\n}\n.chal-name-font{\n  font-size: 14px;\n}\n",""]),e.exports=t},194:function(e,t,n){"use strict";n(117)},195:function(e,t,n){var a=n(13);t=a(!1),t.push([e.i,"\n.labsubnet[data-v-585ba7fe]{\n  border: 1px solid #f76c6c;\n  padding: 3px;\n  border-radius: 5px;\n  background-color: #f76c6c;\n  width: 85%;\n  color: white;\n}\n",""]),e.exports=t},215:function(e,t,n){"use strict";n(126)},216:function(e,t,n){var a=n(13);t=a(!1),t.push([e.i,"\n.vpn-dd-line[data-v-167d5ca5]{\n  border-bottom: 1px solid #000;\n  border-color: #211a52;\n  border-radius: inherit;\n}\n.custom-css[data-v-167d5ca5] {\n  width: 180px;\n  cursor:pointer;\n  color: #211a52;\n}\n",""]),e.exports=t},217:function(e,t,n){"use strict";n.r(t);n(47),n(140),n(149),n(150);var a=n(7),s=n(129),r=(n(151),n(153),n(155),n(157),function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{staticClass:"flag-form"},[n("form",{on:{submit:function(t){return t.preventDefault(),e.submit.apply(null,arguments)}}},[n("span",{staticStyle:{color:"#444"}},[e._v("Flag")]),n("div",{staticClass:"input-div",staticStyle:{"margin-top":"0px !important"}},[n("div",{staticClass:"input-div-icon",class:{"text-success":e.successMsg,"text-danger":e.errorMsg}},[n("i",{staticClass:"fa fa-flag"})]),n("div",{staticClass:"input-div-input"},[n("input",{directives:[{name:"model",rawName:"v-model",value:e.flag,expression:"flag"}],staticClass:"input",attrs:{type:"text",placeholder:"HKN{**********}"},domProps:{value:e.flag},on:{keydown:e.clearMessages,click:e.clearMessages,input:function(t){t.target.composing||(e.flag=t.target.value)}}})])]),n("div",{staticClass:"text-center"},[e.errorMsg?n("p",{staticClass:"text-danger"},[e._v(e._s(e.errorMsg))]):e._e(),e.successMsg?n("p",{staticClass:"text-success"},[e._v(e._s(e.successMsg))]):e._e()]),n("input",{staticClass:"btn btn-login",attrs:{type:"submit",value:"Submit"}})])])}),o=[];r._withStripped=!0;var l=n(10),i=(n(37),n(101),n(18),{name:"FlagChecker",props:{challengeTag:String},data:function(){return{action:"Submit",description:"Flag...",flag:"",errorMsg:"",successMsg:""}},mounted:function(){var e=document.querySelectorAll(".input");function t(){var e=this.parentNode.parentNode;e.classList.add("focus")}function n(){var e=this.parentNode.parentNode;""==this.value&&e.classList.remove("focus")}e.forEach((function(e){e.addEventListener("focus",t),e.addEventListener("blur",n)}))},methods:{clearMessages:function(){this.errorMsg="",this.successMsg=""},submit:function(){var e=Object(l["a"])(regeneratorRuntime.mark((function e(){var t,n,a;return regeneratorRuntime.wrap((function(e){while(1)switch(e.prev=e.next){case 0:return t={method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({tag:this.challengeTag,flag:this.flag})},e.next=3,fetch("/flags/verify",t).then((function(e){return e.json()}));case 3:if(n=e.sent,void 0===n.error){e.next=7;break}return this.errorMsg=n.error,e.abrupt("return");case 7:"ok"===n.status&&(a=this,this.successMsg="You found a flag!",this.flag="",setTimeout((function(){a.$bvModal.hide("challengeModal"),a.$emit("challengeComplete")}),800));case 8:case"end":return e.stop()}}),e,this)})));function t(){return e.apply(this,arguments)}return t}()}}),c=i,u=(n(163),n(2)),d=Object(u["a"])(c,r,o,!1,null,null,null);d.options.__file="src/components/FlagChecker.vue";var p=d.exports,h=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{staticClass:"table-responsive"},[n("table",{staticClass:"table table-striped",attrs:{id:"scoreboardtable"}},[n("thead",{staticClass:"thead-dark-custom text-center"},[n("tr",[n("th",{staticClass:"text-center rank-col"},[e._v("#")]),n("th",{staticClass:"team-col"},[e._v("Team")]),n("th",{staticClass:"score-col"},[e._v("Score")]),e._l(e.challenges,(function(t){return n("th",{key:t.category,staticClass:"scoreboard-border",attrs:{colspan:t.chals.length,id:t.category}},[e._v(" "+e._s(e.category_name(t.category,t.chals.length))+" "),n("b-tooltip",{attrs:{target:t.category,triggers:"hover",placement:"top"}},[e._v(" "+e._s(t.category)+" ")])],1)}))],2),n("tr",[n("th",{staticClass:"rank-col"}),n("th",{staticClass:"team-col"}),n("th",{staticClass:"score-col"}),e._l(e.get_challenges(e.challenges),(function(t){return n("th",{key:t.name,staticClass:"scoreboard-border",attrs:{id:t.name}},[n("span",{staticClass:"chal-points-font"},[e._v(e._s(t.points))]),n("b-tooltip",{attrs:{target:t.name,triggers:"hover",placement:"bottom"}},[e._v(" "+e._s(t.name)+" ")])],1)}))],2)]),e.teams.length>0?n("tbody",e._l(e.teams,(function(e,t){return n("team-row",{key:e.id,attrs:{team:e,pos:t+1}})})),1):n("tbody",[n("tr",{staticClass:"text-center"},[n("td",{attrs:{colspan:e.get_challenges(e.challenges).length+3}},[e._v("No team registered to this event!")])])])])])},f=[];h._withStripped=!0;n(38),n(43),n(44),n(45),n(69),n(70),n(71);var g=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("tr",{class:{"bg-isUser":e.team.is_user}},[e.pos<4?n("td",{staticClass:"text-center rank-col",class:e.get_background(e.pos,e.team.is_user)},[n("span",{staticClass:"icon",class:{"has-text-warning":1===e.pos,"is-silver":2===e.pos,"is-bronze":3===e.pos}},[n("i",{staticClass:"fas fa-trophy"})])]):n("td",{staticClass:"text-center rank-col",class:e.get_background(e.pos,e.team.is_user)},[e._v(e._s(e.pos))]),n("td",{staticClass:"team-col",class:e.get_background(e.pos,e.team.is_user)},[n("strong",[e._v(e._s(e.team.name))])]),n("td",{staticClass:"text-center score-col",class:e.get_background(e.pos,e.team.is_user)},[e._v(e._s(e.team.tpoints))]),e._l(e.team.completions,(function(t){return n("challenge-cell",{key:t,class:e.get_background(e.pos,e.team.is_user),attrs:{completed:null!=t}})}))],2)},m=[];g._withStripped=!0;n(178);var b=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("td",{staticClass:"text-center scoreboard-border"},[n("span",{staticClass:"icon",class:{"text-success":e.completed}},[n("i",{staticClass:"fas",class:{"fa-check-circle":e.completed,"fa-circle":!e.completed}})])])},v=[];b._withStripped=!0;var _={name:"challenge-cell",props:{completed:Boolean}},y=_,C=Object(u["a"])(y,b,v,!1,null,null,null);C.options.__file="src/components/ChallengeCell.vue";var w=C.exports,x={name:"team-row",props:{team:Object,pos:Number},components:{ChallengeCell:w},methods:{get_background:function(e,t){return t?"bg-isUser":e%2===0?"even":"odd"}}},S=x,k=(n(182),Object(u["a"])(S,g,m,!1,null,null,null));k.options.__file="src/components/TeamRow.vue";var M=k.exports,T={name:"scoreboard",data:function(){return{teams:[],challenges:[]}},created:function(){var e=new URL("/scores",window.location.href);e.protocol=e.protocol.replace("http","ws"),this.connectToWS(e.href)},methods:{get_challenges:function(e){var t=[];for(var n in e)for(var a in e[n].chals)t.push(e[n].chals[a]);return t},category_name:function(e,t){if(t>3)return e;switch(e){case"Web exploitation":return"Web E.";case"Forensics":return"For..";case"Cryptography":return"Cry..";case"Binary":return"Bin..";case"Reverse Engineering":return"R. Eng."}},connectToWS:function(e){var t=this,n=new WebSocket(e);n.onmessage=t.receiveMsg,n.onclose=function(){n=null,setTimeout((function(){t.connectToWS(e)}),3e3)}},receiveMsg:function(e){for(var t=e.data.split("\n"),n=0;n<t.length;n++){var a=t[n],s=JSON.parse(a);"scoreboard"==s.msg&&(this.challenges=s.values.challenges,this.teams=s.values.teams.sort((function(e,t){return t.tpoints-e.tpoints})))}}},components:{TeamRow:M}},O=T,D=(n(184),Object(u["a"])(O,h,f,!1,null,null,null));D.options.__file="src/components/Scoreboard.vue";var E=D.exports,j=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{style:[this.isLabAssigned?{}:{"pointer-events":"none",opacity:"0.5"}],attrs:{id:"challenges-board"}},[e._l(e.challengesFromAmigo,(function(t){return n("div",{key:t[0].challenge.category,staticClass:"row mt-2"},[n("div",{staticClass:"category-header col-md-12 mb-3"},[n("h3",[e._v(e._s(t[0].challenge.category))])]),e._l(t,(function(t){return n("div",{key:t.challenge.tag,staticClass:"col-lg-3 col-md-4"},[n("button",{staticClass:"btn challenge-button w-100 text-truncate pt-3 pb-3 mb-2",class:{"btn-success":t.isUserCompleted,"btn-haaukins":!t.isUserCompleted},on:{click:function(n){return e.openModal(t)}}},[n("p",{staticClass:"chal-name-font"},[e._v(e._s(t.challenge.name))]),n("span",[e._v(e._s(t.challenge.points))])])])}))],2)})),n("challenge-modal",{attrs:{challenge:this.chalInfo,teamsCompleted:this.teamsCompleted},on:{runChallenge:e.runChallenge,resetChallenge:e.resetChallenge,challengeCompleteReload:e.challengeCompleteReload}})],2)},R=[];j._withStripped=!0;var $=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",[n("b-modal",{ref:"modal",attrs:{id:"challengeModal",centered:"","hide-footer":"","hide-header":""}},[n("div",{staticClass:"modal-body"},[n("button",{staticClass:"close",attrs:{type:"button"},on:{click:function(t){return e.$bvModal.hide("challengeModal")}}},[n("span",{attrs:{"aria-hidden":"true"}},[e._v("×")])]),n("nav",[n("div",{staticClass:"nav nav-tabs",attrs:{id:"nav-tab",role:"tablist"}},[n("a",{staticClass:"nav-item nav-link active",attrs:{id:"nav-challenge-tab","data-toggle":"tab",href:"#nav-challenge",role:"tab","aria-controls":"nav-challenge","aria-selected":"true"}},[e._v("Challenge")]),n("a",{staticClass:"nav-item nav-link",attrs:{id:"nav-solves-tab","data-toggle":"tab",href:"#nav-solves",role:"tab","aria-controls":"nav-solves","aria-selected":"false"}},[e._v(e._s(e.checkTeams(e.teamsCompleted))+" Solves")]),e.challenge.staticChallenge?e._e():n("ResetChallenge",{attrs:{challengeTag:e.challenge.tag},on:{resetChallenge:function(t){return e.$emit("resetChallenge")}}}),e.challenge.staticChallenge?e._e():n("RunChallenge",{attrs:{challengeTag:e.challenge.tag},on:{runChallenge:function(t){return e.$emit("runChallenge")}}})],1)]),n("div",{staticClass:"tab-content"},[n("div",{staticClass:"tab-pane fade show active",attrs:{id:"nav-challenge",role:"tabpanel","aria-labelledby":"nav-challenge-tab"}},[n("h2",{staticClass:"chal-name text-center pt-5 pb-1"},[e._v(e._s(e.challenge.name))]),n("h4",{staticClass:"chal-value text-center mb-5"},[e._v(e._s(e.challenge.points))]),n("span",{staticClass:"chal-desc mb-5"},[n("p",{domProps:{innerHTML:e._s(e.challenge.teamDescription)}})]),n("FlagChecker",{staticClass:"mt-5",attrs:{challengeTag:e.challenge.tag},on:{challengeComplete:function(t){return e.$emit("challengeCompleteReload")}}})],1),n("div",{staticClass:"tab-pane fade",attrs:{id:"nav-solves",role:"tabpanel","aria-labelledby":"nav-solves-tab"}},[n("table",{staticClass:"table table-striped text-center mt-4"},[n("thead",{staticClass:"thead-dark-custom"},[n("tr",[n("th",[n("b",[e._v("Name")])]),n("th",[n("b",[e._v("Date")])])])]),0!=e.checkTeams(e.teamsCompleted)?n("tbody",e._l(e.teamsCompleted,(function(t){return n("tr",{key:t.teamName},[n("td",[e._v(e._s(t.teamName))]),n("td",[e._v(e._s(e.beauty_date(t.completedAt)))])])})),0):n("tbody",[n("tr",[n("td",{attrs:{colspan:"2"}},[e._v("Nobody solved this challenge!")])])])])])])])])],1)},N=[];$._withStripped=!0;var P=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{staticClass:"pl-3"},[n("form",{on:{submit:function(t){return t.preventDefault(),e.submit.apply(null,arguments)}}},[n("input",{staticClass:"btn btn-haaukins",class:{"btn-danger":e.isError,"btn-success":e.isSuccess},staticStyle:{width:"auto"},attrs:{type:"submit",disabled:e.isDisabled,value:"Reset"}})])])},L=[];P._withStripped=!0;var A={name:"ResetChallenge",props:{challengeTag:String},data:function(){return{isDisabled:!1,isError:!1,isSuccess:!1}},methods:{submit:function(){var e=Object(l["a"])(regeneratorRuntime.mark((function e(){var t,n,a;return regeneratorRuntime.wrap((function(e){while(1)switch(e.prev=e.next){case 0:return this.isDisabled=!0,t={method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({tag:this.challengeTag})},e.next=4,fetch("/reset/challenge",t).then((function(e){return e.json()}));case 4:if(n=e.sent,void 0===n.error){e.next=9;break}return this.isError=!0,this.isDisabled=!1,e.abrupt("return");case 9:"ok"===n.status&&(a=this,this.isSuccess=!0,this.isDisabled=!1,setTimeout((function(){a.$bvModal.hide("challengeModal"),a.$emit("resetChallenge")}),1e3));case 10:case"end":return e.stop()}}),e,this)})));function t(){return e.apply(this,arguments)}return t}()}},F=A,W=Object(u["a"])(F,P,L,!1,null,null,null);W.options.__file="src/components/ResetChallenge.vue";var I=W.exports,B=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{staticClass:"pl-3"},[n("form",{on:{submit:function(t){return t.preventDefault(),e.submit.apply(null,arguments)}}},[n("input",{staticClass:"btn btn-haaukins",class:{"btn-danger":e.isError,"btn-success":e.isSuccess},staticStyle:{width:"auto"},attrs:{type:"submit",disabled:e.isDisabled,value:"Start/Stop"}})])])},J=[];B._withStripped=!0;var U={name:"RunChallenge",props:{challengeTag:String},data:function(){return{isDisabled:!1,isError:!1,isSuccess:!1}},methods:{submit:function(){var e=Object(l["a"])(regeneratorRuntime.mark((function e(){var t,n,a;return regeneratorRuntime.wrap((function(e){while(1)switch(e.prev=e.next){case 0:return this.isDisabled=!0,t={method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({tag:this.challengeTag})},e.next=4,fetch("/manage/challenge",t).then((function(e){return e.json()}));case 4:if(n=e.sent,void 0===n.error){e.next=9;break}return this.isError=!0,this.isDisabled=!1,e.abrupt("return");case 9:"ok"===n.status&&(a=this,this.isSuccess=!0,this.isDisabled=!1,setTimeout((function(){a.$bvModal.hide("challengeModal"),a.$emit("runChallenge")}),1e3));case 10:case"end":return e.stop()}}),e,this)})));function t(){return e.apply(this,arguments)}return t}()}},H=U,V=Object(u["a"])(H,B,J,!1,null,null,null);V.options.__file="src/components/RunChallenge.vue";var G=V.exports,K={name:"ChallengeModal",components:{ResetChallenge:I,FlagChecker:p,RunChallenge:G},props:{challenge:Object,teamsCompleted:Array},data:function(){return{}},methods:{checkTeams:function(e){return null!=e?e.length:0},beauty_date:function(e){var t=new Date(e),n=["January","February","March","April","May","June","July","August","September","October","November","December"];return t.getHours()+":"+t.getMinutes()+"   "+t.getDate()+" "+n[t.getMonth()]}}},z=K,Y=(n(186),Object(u["a"])(z,$,N,!1,null,"eec7658c",null));Y.options.__file="src/components/ChallengeModal.vue";var q=Y.exports,Q={name:"ChallegesPage",components:{ChallengeModal:q},data:function(){return{chalInfo:{},isLabAssigned:!1,teamsCompleted:[],challengesFromAmigo:[]}},created:function(){this.connectToWS()},methods:{sortChallenges:function(){var e={};for(var t in this.challengesFromAmigo.forEach((function(t){t.challenge.category in e||(e[t.challenge.category]=[]),e[t.challenge.category].push(t)}),this),e)e[t]=e[t].sort((function(e,t){return e.challenge.points-t.challenge.points}));this.challengesFromAmigo=e},openModal:function(e){this.chalInfo=e.challenge,this.teamsCompleted=e.teamsCompleted,this.isLabAssigned&&this.$bvModal.show("challengeModal")},connectToWS:function(){var e=new URL("/challengesFrontend",window.location.href);e.protocol=e.protocol.replace("http","ws");var t=this,n=new WebSocket(e);n.onmessage=t.receiveMsg,n.onclose=function(){n=null,setTimeout((function(){t.connectToWS(e)}),3e3)}},receiveMsg:function(e){for(var t=e.data.split("\n"),n=0;n<t.length;n++){var a=t[n],s=JSON.parse(a);"challenges"===s.msg&&(this.challengesFromAmigo=s.values,this.isLabAssigned=s.isLabAssigned)}this.sortChallenges()},challengeCompleteReload:function(){this.connectToWS()},runChallenge:function(){this.connectToWS()},resetChallenge:function(){this.connectToWS()}}},X=Q,Z=(n(188),Object(u["a"])(X,j,R,!1,null,null,null));Z.options.__file="src/components/ChallegesPage.vue";var ee=Z.exports,te=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{attrs:{id:"teams"}},[n("Plotly",{attrs:{data:e.traces,layout:e.layout}}),n("table",{staticClass:"table table-striped mt-5"},[e._m(0),e.teams.length>0?n("tbody",e._l(e.teams,(function(t,a){return n("tr",{key:t.id},[n("td",{staticClass:"text-center"},[e._v(e._s(a+1))]),n("td",[e._v(e._s(t.name))]),n("td",[e._v(e._s(t.tpoints))])])})),0):n("tbody",[e._m(1)])])],1)},ne=[function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("thead",{staticClass:"thead-dark-custom"},[n("tr",[n("th",{staticClass:"text-center"},[e._v("#")]),n("th",[e._v("Team")]),n("th",[e._v("Score")])])])},function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("tr",{staticClass:"text-center"},[n("td",{attrs:{colspan:"3"}},[e._v("No team registered to this event!")])])}];te._withStripped=!0;n(190),n(191),n(192),n(193);var ae=n(127),se={name:"ScoreGraph",components:{Plotly:ae["Plotly"]},data:function(){return{teams:[],data:[{x:[1,2,3,4],y:[10,15,13,17],type:"scatter"}],traces:[],layout:{paper_bgcolor:"rgba(0,0,0,0)",plot_bgcolor:"rgba(0,0,0,0)",hovermode:"closest",xaxis:{showgrid:!1,showspikes:!0},yaxis:{showgrid:!1,showspikes:!0},legend:{orientation:"h"}}}},created:function(){var e=new URL("/challengesFrontend",window.location.href);e.protocol=e.protocol.replace("http","ws"),this.connectToWS(e.href)},methods:{connectToWS:function(e){var t=this,n=new WebSocket(e);n.onmessage=t.receiveMsg,n.onclose=function(){n=null,setTimeout((function(){t.connectToWS(e)}),3e3)}},receiveMsg:function(e){for(var t=e.data.split("\n"),n=0;n<t.length;n++){var a=t[n],s=JSON.parse(a);"scoreboard"===s.msg&&(this.teams=s.values.teams.sort((function(e,t){return t.tpoints-e.tpoints})),this.sortSolvedData())}},sortSolvedData:function(){for(var e=0;e<this.teams.length;e++){for(var t=[],n=0;n<this.teams[e].completions.length;n++)t.push({date:this.teams[e].completions[n],points:this.teams[e].points[n]});t.sort((function(e,t){return new Date(e.date)-new Date(t.date)}));for(var a=0;a<t.length;a++)this.teams[e].completions[a]=t[a].date,this.teams[e].points[a]=t[a].points}this.scoreGraph()},scoreGraph:function(){for(var e=0;e<this.teams.length;e++){for(var t=[],n=[],a=0;a<this.teams[e].completions.length;a++)if(null!=this.teams[e].completions[a]){var s=new Date(this.teams[e].completions[a]);n.push(s),t.push(this.teams[e].points[a])}t=this.cumulativeSum(t);var r={x:n,y:t,mode:"lines+markers",name:this.teams[e].name,marker:{color:this.colorHash(this.teams[e].name+this.teams[e].id)},line:{color:this.colorHash(this.teams[e].name+this.teams[e].id)}};this.traces.push(r)}this.traces.sort((function(e,t){var n=t["y"][t["y"].length-1]-e["y"][e["y"].length-1];return n||e["x"][e["x"].length-1]-t["x"][t["x"].length-1]}))},cumulativeSum:function(e){for(var t=e.concat(),n=0;n<e.length;n++)t[n]=e.slice(0,n+1).reduce((function(e,t){return e+t}));return t},colorHash:function(e){for(var t=0,n=0;n<e.length;n++)t=e.charCodeAt(n)+((t<<5)-t);for(var a="#",s=0;s<3;s++){var r=t>>8*s&255;a+=("00"+r.toString(16)).substr(-2)}return a}}},re=se,oe=Object(u["a"])(re,te,ne,!1,null,"306fd2e5",null);oe.options.__file="src/components/TeamsPage.vue";var le=oe.exports,ie=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{staticClass:"mt-toppage-reset pt-2 mb-2"},[0==e.isVpn?n("form",{on:{submit:function(t){return t.preventDefault(),e.submit.apply(null,arguments)}}},[n("input",{staticClass:"btn btn-login",staticStyle:{width:"auto"},attrs:{type:"submit",disabled:e.isDisabled,value:"RESET Kali Machine"}})]):1==e.isVpn?n("div",[n("div",{staticClass:"labsubnet"},[e._v(" Lab Subnet: [ "),n("b",[e._v(e._s(e.labSubnet)+" ")]),e._v("] ")])]):n("div",[n("form",{on:{submit:function(t){return t.preventDefault(),e.submit.apply(null,arguments)}}},[n("input",{staticClass:"btn btn-login",staticStyle:{width:"auto"},attrs:{type:"submit",disabled:e.isDisabled,value:"RESET Kali Machine"}})]),n("div",{staticClass:"labsubnet mt-2"},[e._v(" Lab Subnet: [ "),n("b",[e._v(e._s(e.labSubnet)+" ")]),e._v("] ")])])])},ce=[];ie._withStripped=!0;var ue={name:"ResetFrontend",data:function(){return{isDisabled:!1,isVpn:0,labSubnet:""}},created:function(){this.getLabSubnet()},methods:{submit:function(){var e=Object(l["a"])(regeneratorRuntime.mark((function e(){var t,n,a;return regeneratorRuntime.wrap((function(e){while(1)switch(e.prev=e.next){case 0:return this.isDisabled=!0,t={method:"POST",headers:{"Content-Type":"application/json"}},e.next=4,fetch("/reset/frontend",t).then((function(e){return e.json()}));case 4:if(n=e.sent,a=document.getElementById("reset-frontend-resp"),void 0===n.error){e.next=10;break}return a.innerHTML='<span class="text-danger">'+n.error+"</span>",this.isDisabled=!1,e.abrupt("return");case 10:"ok"===n.status&&(a.innerHTML='<span class="text-success">Kali Machine successfully restarted</span>',this.isDisabled=!1);case 11:case"end":return e.stop()}}),e,this)})));function t(){return e.apply(this,arguments)}return t}(),getLabSubnet:function(){var e=Object(l["a"])(regeneratorRuntime.mark((function e(){var t,n;return regeneratorRuntime.wrap((function(e){while(1)switch(e.prev=e.next){case 0:return t={method:"POST",headers:{"Content-Type":"application/json"}},e.next=3,fetch("/get/labsubnet",t).then((function(e){return e.json()}));case 3:n=e.sent,this.isVpn=n.isVPN,""==n.labSubnet?this.labSubnet="NOT ASSIGNED YET":this.labSubnet=n.labSubnet+"/24";case 6:case"end":return e.stop()}}),e,this)})));function t(){return e.apply(this,arguments)}return t}()}},de=ue,pe=(n(194),Object(u["a"])(de,ie,ce,!1,null,"585ba7fe",null));pe.options.__file="src/components/ResetFrontend.vue";var he=pe.exports,fe=function(){var e=this,t=e.$createElement,n=e._self._c||t;return n("div",{staticClass:"dropdown"},[n("button",{staticClass:"btn btn-haaukins dropdown-toggle dropdown-css",attrs:{type:"button",id:"dropdownMenuButton","data-toggle":"dropdown"},on:{click:function(t){return e.createDropDown()}}},[e._v(" VPN ")]),n("div",{staticClass:"dropdown-menu custom-css",attrs:{"aria-labelledby":"dropdownMenuButton"}},e._l(e.dropDownList,(function(t){return n("a",{key:t.vpnConnID,staticClass:"dropdown-item vpn-dd-line",on:{click:function(n){return e.downloadConf(t.vpnConnID,t.status)}}},[e._v(" "+e._s(t.vpnConnID)+" "),n("span",{staticClass:"float-right"},[e._v(e._s(t.status))])])})),0)])},ge=[];fe._withStripped=!0;var me=n(196).default,be={name:"VPNDropdown",data:function(){return{dropDownList:[]}},methods:{createDropDown:function(){var e=Object(l["a"])(regeneratorRuntime.mark((function e(){var t,n,a;return regeneratorRuntime.wrap((function(e){while(1)switch(e.prev=e.next){case 0:return this.dropDownList=[],t={method:"POST",headers:{"Content-Type":"application/json"}},e.next=4,fetch("/vpn/status",t).then((function(e){return e.json()}));case 4:for(n=e.sent,a=0;a<n.length;a++)this.dropDownList.push(n[a]);case 6:case"end":return e.stop()}}),e,this)})));function t(){return e.apply(this,arguments)}return t}(),downloadConf:function(e,t){me({url:"/vpn/download",method:"POST",responseType:"blob",data:{vpnConnID:e,status:t}}).then((function(t){var n=window.URL.createObjectURL(new Blob([t.data])),a=document.createElement("a");a.href=n,a.setAttribute("download",e+".conf"),document.body.appendChild(a),a.click()}))}}},ve=be,_e=(n(215),Object(u["a"])(ve,fe,ge,!1,null,"167d5ca5",null));_e.options.__file="src/components/VPNDropdown.vue";var ye=_e.exports;a["default"].use(s["a"]),a["default"].config.productionTip=!1,document.getElementById("vpn-dropdown")&&new a["default"]({render:function(e){return e(ye)}}).$mount("#vpn-dropdown"),document.getElementById("vpn-dropdown-2")&&new a["default"]({render:function(e){return e(ye)}}).$mount("#vpn-dropdown-2"),document.getElementById("reset-frontend")&&new a["default"]({render:function(e){return e(he)}}).$mount("#reset-frontend"),document.getElementById("flagchecker")&&new a["default"]({render:function(e){return e(p)}}).$mount("#flagchecker"),document.getElementById("scoreboard")&&new a["default"]({render:function(e){return e(E)}}).$mount("#scoreboard"),document.getElementById("challenges")&&new a["default"]({render:function(e){return e(ee)}}).$mount("#challenges"),document.getElementById("teamspagevue")&&new a["default"]({render:function(e){return e(le)}}).$mount("#teamspagevue")}});