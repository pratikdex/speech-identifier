import Image from "next/image";

export const Header = () => {
  return <div className="flex items-center justify-between bg-black p-5">
    <Image src="/adalat-logo.svg" height={36} width={108} alt="Adalat AI Logo" />
  </div>;
};